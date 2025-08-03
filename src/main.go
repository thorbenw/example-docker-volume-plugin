package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/thorbenw/example-docker-volume-plugin/metric"
	"github.com/thorbenw/example-docker-volume-plugin/mount"
	"github.com/thorbenw/example-docker-volume-plugin/proc"
	"github.com/thorbenw/example-docker-volume-plugin/utils"
	"golang.org/x/exp/maps"
)

const (
	VERSION_DEVEL                                 = "(devel)"
	VOLUME_PROCESS_OPTIONS_ENV                    = "VOLUME_PROCESS_OPTIONS"
	VOLUME_PROCESS_OPTIONS_SEPARATOR              = "&"
	VOLUME_PROCESS_OPTIONS_MOUNTPOINT_PLACEHOLDER = "{mountPoint}"
	MOUNT_OPTIONS_ENV                             = "MOUNT_OPTIONS"
	// The default folder for plugin socket files. Unfortunately, github.com/docker/go-plugins-helpers/sdk.pluginSockDir is NOT exported :(
	DEFAULT_PLUGIN_SOCK_DIR = "/run/docker/plugins"
	DEFAULT_LOG_LEVEL       = slog.LevelInfo
	EXIT_CODE_OK            = 0
	EXIT_CODE_ERROR         = 1
	EXIT_CODE_USAGE         = 2
	EXIT_CODE_PARAM         = 3
	EXIT_CODE_HELP          = 127
)

var (
	version         = VERSION_DEVEL
	lastLogLevel    = DEFAULT_LOG_LEVEL
	logLevelStrings = map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	logLevelKeys = map[slog.Level]string{
		slog.LevelDebug: "dbg",
		slog.LevelInfo:  "inf",
		slog.LevelWarn:  "wrn",
		slog.LevelError: "err",
	}
)

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		return slog.Attr{}
	case slog.LevelKey:
		if level, ok := a.Value.Any().(slog.Level); ok {
			lastLogLevel = level
		}

		return slog.Attr{}
	case slog.MessageKey:
		key, ok := logLevelKeys[lastLogLevel]
		if !ok {
			key = "unk"
		}
		lastLogLevel = DEFAULT_LOG_LEVEL

		return slog.Attr{Key: key, Value: a.Value}
	default:
		return a
	}
}

func os_LookupEnv(key string) (string, bool) {
	return os.LookupEnv(strings.ToUpper(strings.Replace(key, "-", "_", -1)))
}

func flags_String(flags *flag.FlagSet, name string, usage string, value string) (result *string) {
	result = flags.String(name, value, usage)

	if env, ok := os_LookupEnv(name); ok {
		*result = env
	}

	return
}

func flags_Bool(flags *flag.FlagSet, name string, usage string, value bool) (result *bool) {
	result = flags.Bool(name, value, usage)

	if env, ok := os_LookupEnv(name); ok {
		if b, err := strconv.ParseBool(env); err == nil {
			*result = b
		}
	}

	return
}

func flags_Uint(flags *flag.FlagSet, name string, usage string, value uint) (result *uint) {
	result = flags.Uint(name, value, usage)

	if env, ok := os_LookupEnv(name); ok {
		if b, err := strconv.ParseUint(env, 10, 32); err == nil {
			*result = uint(b)
		}
	}

	return
}

//go:test exclude
func main() {
	args := os.Args[1:] // w/o program name, which is in element 0

	os.Exit(entryPoint(os.Args[0], args))
}

func entryPoint(arg0 string, args []string) (exitCode int) {
	usageMsg := fmt.Sprintf("Usage: %s [OPTIONS]\n", arg0)
	logLevelList := strings.Join(maps.Keys(logLevelStrings), " | ")
	volumeProcessRecoveryModeList := strings.Join(utils.Select(maps.Values(proc.RecoveryModeNames()), strings.ToLower), " | ")

	var (
		env string
		ok  bool
	)

	flags := flag.NewFlagSet(arg0, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		w := flags.Output()

		fmt.Fprintln(w)
		fmt.Fprintln(w, usageMsg)
		fmt.Fprintln(w, "Options:")
		flags.PrintDefaults()
		fmt.Fprintln(w)
	}

	showHelp := flags.Bool("help", false, "Display usage information. If present, all other options are ignored.")
	showVersion := flags.Bool("version", false, "Display version and exit. Ignores all other options except --help.")
	showBuildInfo := flags.Bool("build-info", false, "Display go build information and exit. Ignores all other options except --help and --version.")

	logLevelString := flags_String(flags, "log-level", fmt.Sprintf("The log level (one out of %s).", logLevelList), "info")
	logSource := flags_Bool(flags, "log-source", "Include the source code position in the log.", false)
	propagatedMount := flags_String(flags, "propagated-mount", "Where to find the propagated mount.", "/data")

	volumeProcessBinary := flags_String(flags, "volume-process-binary", "Executable file to run for each volume. Must be either an absolute file path or be within $PATH.", "")
	volumeProcessRecoveryModeString := flags_String(flags, "volume-process-recovery-mode", fmt.Sprintf("How to behave if the volume process terminates unexpectedly (one out of %s).", volumeProcessRecoveryModeList), strings.ToLower(proc.RecoveryModeIgnore.String()))
	volumeProcessRecoveryMaxPerMin := flags_Uint(flags, "volume-process-recovery-max-per-min", "How many times the volume process will be restarted before giving up.", 3)
	volumeProcessOptions := proc.NewOptions(5, VOLUME_PROCESS_OPTIONS_SEPARATOR, true)
	env, ok = os_LookupEnv(VOLUME_PROCESS_OPTIONS_ENV)
	if !ok || strings.TrimSpace(env) == "" {
		env = VOLUME_PROCESS_OPTIONS_MOUNTPOINT_PLACEHOLDER
	}
	if err := volumeProcessOptions.Set(env); err != nil {
		fmt.Fprintln(flags.Output(), err)
		return EXIT_CODE_ERROR
	}
	flags.Var(volumeProcessOptions, "c", fmt.Sprintf("Command line options for the volume process, separated by '%s' (without the single quotation marks). Ocurrences of '%s' (again, without the single quotation marks) will be replaced with the mount point path.", VOLUME_PROCESS_OPTIONS_SEPARATOR, VOLUME_PROCESS_OPTIONS_MOUNTPOINT_PLACEHOLDER))

	mountOptionsVolumeProcessOptions := flags_String(flags, "mount-options-volume-process-option", "A command line option for the volume process that is inserted before the mount options (most tools e.g. use '-o' (without the single quotation marks)).", "")
	mountOptions := mount.NewOptions(10)
	env, ok = os_LookupEnv(MOUNT_OPTIONS_ENV)
	if ok {
		if err := mountOptions.Set(env); err != nil {
			fmt.Fprintln(flags.Output(), err)
			return EXIT_CODE_ERROR
		}
	}
	flags.Var(mountOptions, "o", "Mount options as used in mtab.")

	if err := flags.Parse(args); err != nil {
		return EXIT_CODE_USAGE
	}

	if *showHelp {
		fmt.Fprintf(flags.Output(), "%s, version %s\n", arg0, version)
		flags.Usage()
		return EXIT_CODE_HELP
	}

	if *showVersion {
		fmt.Fprintf(os.Stdout, "%s\n", version)
		return EXIT_CODE_OK
	}

	if *showBuildInfo {
		if buildInfo, ok := debug.ReadBuildInfo(); !ok {
			fmt.Fprintf(flags.Output(), "Failed to read build info.")
			return EXIT_CODE_ERROR
		} else {
			fmt.Fprintln(os.Stdout, buildInfo)
			return EXIT_CODE_OK
		}
	}

	errors := []string{}

	if f, err := os.Lstat(*propagatedMount); err != nil {
		errors = append(errors, fmt.Sprintf("The propagated mount folder [%s] is not accessible (%s).", *propagatedMount, err.Error()))
	} else {
		if !f.IsDir() {
			errors = append(errors, fmt.Sprintf("The propagated mount folder [%s] is not a directory (type is %s).", *propagatedMount, f.Mode().Type().String()))
		}
	}

	var logLevel slog.Level
	if l, ok := logLevelStrings[*logLevelString]; !ok {
		errors = append(errors, fmt.Sprintf("Log level [%s] is not valid (use one out of %s).", *logLevelString, logLevelList))
	} else {
		logLevel = l
	}

	var invalidRecoveryMode = proc.RecoveryMode(-1)
	var volumeProcessRecoveryMode proc.RecoveryMode
	if volumeProcessRecoveryMode = proc.RecoveryModeParse(*volumeProcessRecoveryModeString, invalidRecoveryMode); volumeProcessRecoveryMode == invalidRecoveryMode {
		errors = append(errors, fmt.Sprintf("Volume process recovery mode [%s] is not valid (use one out of %s).", *volumeProcessRecoveryModeString, volumeProcessRecoveryModeList))
	} else {
		if volumeProcessRecoveryMode == proc.RecoveryModeRestart && *volumeProcessRecoveryMaxPerMin < 1 {
			errors = append(errors, fmt.Sprintf("Volume process recovery rate limit must not be less than 1 with volume process recovery mode is [%s] (specified value is %d).", *volumeProcessRecoveryModeString, *volumeProcessRecoveryMaxPerMin))
		}
	}

	if l := len(errors); l > 0 {
		fmt.Fprintf(os.Stderr, "%s found %d errors during parameter and configuration checks:\n", arg0, l)

		for i, v := range errors {
			fmt.Fprintf(os.Stderr, "\t%d. %s\n", i+1, v)
		}

		fmt.Fprintln(os.Stderr)

		return EXIT_CODE_PARAM
	}

	logopt := slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: replaceAttr,
		AddSource:   *logSource,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &logopt))
	logger.Info("Starting the Example Docker Volume Plugin.", "version", version, "args", args)
	proc.Logger = logger

	driver, err := exampleDriver_New(*propagatedMount, *logger)
	if err == nil {
		if strings.TrimSpace(*volumeProcessBinary) != "" {
			if binaryPath, err := exec.LookPath(*volumeProcessBinary); err != nil {
				logger.Error(err.Error())
				return EXIT_CODE_ERROR
			} else {
				driver.GetVolumeProcess = func() (*exec.Cmd, *proc.Options, *mount.Options) {
					return exec.Command(binaryPath), &volumeProcessOptions, &mountOptions
				}
				driver.SetVolumeProcessOptions = func(cmd *exec.Cmd, vpOpt *proc.Options, mOpt *mount.Options, mountPoint string) error {
					if vpOpt != nil && vpOpt.Len() > 0 {
						lvpOpt := utils.Select(vpOpt.Slice(), func(str string) string {
							return strings.ReplaceAll(str, VOLUME_PROCESS_OPTIONS_MOUNTPOINT_PLACEHOLDER, mountPoint)
						})
						cmd.Args = append(cmd.Args, lvpOpt...)
					}

					if mOpt != nil && mOpt.Len() > 0 {
						*mountOptionsVolumeProcessOptions = strings.TrimSpace(*mountOptionsVolumeProcessOptions)
						if *mountOptionsVolumeProcessOptions != "" {
							cmd.Args = append(cmd.Args, *mountOptionsVolumeProcessOptions)
						}
						cmd.Args = append(cmd.Args, mOpt.String())
					}

					return nil
				}
			}
		}
		driver.VolumeProcessRecoveryMode = volumeProcessRecoveryMode
		driver.VolumeProcessRecoveryRateLimit = &metric.MetricRateLimit{Limit: *volumeProcessRecoveryMaxPerMin, Duration: time.Minute}
		logger.Debug(fmt.Sprintf("Created driver %T.", driver), "driver", driver)
	} else {
		logger.Error(err.Error())
		return EXIT_CODE_ERROR
	}

	handler := volume.NewHandler(*driver)
	logger.Debug(fmt.Sprintf("Created handler %T.", handler), "handler", handler)

	user, err := user.Lookup("root")
	if err == nil {
		logger.Debug(fmt.Sprintf("Fetched user %T", user), "user", user)
	} else {
		logger.Error(err.Error())
		return EXIT_CODE_ERROR
	}

	gid, err := strconv.Atoi(user.Gid)
	if err == nil {
		logger.Debug(fmt.Sprintf("Fetched gid %T", gid), "gid", gid)
	} else {
		logger.Error(err.Error())
		return EXIT_CODE_ERROR
	}

	if err := handler.ServeUnix("example", gid); err != nil {
		logger.Error(fmt.Sprintf("Calling %T.ServeUnix() failed.", handler), "err", err)
		return EXIT_CODE_ERROR
	}

	return EXIT_CODE_OK
}
