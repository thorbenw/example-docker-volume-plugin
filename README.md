Example Docker Volume Plugin
============================

This is another example driver like others present on
[GitHub](https://github.com) using the
[official Plugins Helpers](https://github.com/docker/go-plugins-helpers) like
the [example shim driver](https://github.com/docker/go-plugins-helpers/blob/main/volume/shim/shim.go),
[marcelo-ochoa/docker-volume-plugins](https://github.com/marcelo-ochoa/docker-volume-plugins),
[fntlnz/docker-volume-plugin-example](https://github.com/fntlnz/docker-volume-plugin-example),
[vieux/docker-volume-sshfs](https://github.com/vieux/docker-volume-sshfs),
[sapk/docker-volume-rclone](https://github.com/sapk/docker-volume-rclone)
and others.

# Basic features
- Version 2.0 Docker plugin
- No 3rd party package dependencies
- Supports volume processes, i.e. for each volume, a process can be maintained
  (e.g. started on volume creation, restarted on crash and terminated on volume
  removal.)

# Usage

## Options
There are different kinds of options that must be distinguished, some of which
also can be specified on different levels. The _plugin level_ comprises of command
line options and environment variables as the 'highest' level. The _volume
level_, coming in charge on volume creation, is the 'lower' level, which for
most occasions takes precedence over the plugin level.

### Plugin Options
Options for a plugin that control plugin (and therefore driver) behaviour. To
see the list of and get help for available options in this plugin, call it's
binary with the `--help` option.

Plugin options can only be specified on plugin level.

### Volume Options
These options can have different values for each volume and are also persisted
in the control file. Volume options are key-value pairs and do not accept empty
values.

Volume options can only be specified on volume level.

### Volume Process Options
Command line parameters for the volume process binary.

Volume process options can be specified on plugin and volume level and are
additive, i.e. specifications from both levels are merged, but without handling
duplicates or any removal strategy.

### Mount Options
When mounting a file system using `mtab`, options are specified as a comma
separated list of key-value pairs and flags (keys without values).
Most tools used for backing Docker volume plugins not only support this format,
but even organize their options in that way. Therefore, this plugin supports
managing mount options and eventually forwaring them to volume processes.

Mount options can also be specified on both, plugin and volume level. Option
values provided on volume level take precedence over option values provided on
plugin level. Options already set on plugin level can be unset by providing `-`
as their value on volume level.

### Plugin Level Specification
On this specification level, option values are provided as command line options
when calling the plugin's binary.

Values for all options provided with this plugin, except `--help`, `--version`,
`--build-info`, `-c` and `-o`, can also be provided using environment variables
whose names result from converting all characters to upper case, removing all
leading dashes (`-`) and replacing all remaining dashes to underscores (`_`).
E.g. the value of the `--volume-process-recovery-mode` option can be also set
using a `VOLUME_PROCESS_RECOVERY_MODE` environment variable.
A value for the `-c` option can be set using a `VOLUME_PROCESS_OPTIONS`
environment variable, and a value for the `-o` option can be set using a
`MOUNT_OPTIONS` environment variable.
If values for a single option are specified in both, environment variables as
well as command line options, those specified on the command line take
precedence and the values from the environment variables are ignored.

In order to support volume process options, there is a plugin option `-c`, which
can be used to provide default volume process option values, which must be
specified in a single string using `&` in the form
`-c=-flag&--option=value&another expression`.

In order to support mount options, there is a plugin option `-o`, which can be
used to provide default mount option values. These must be specified as a single
string in the form `-o=key1=value1,key2=value2,flag`.

### Volume Level Specification
When creating a volume, options can be specified using the `-o` option of the
`docker volume create` command.

In order to support mount options as well as volume process options, volume
options with key `o` are processed as mount options, and volume options with key
`c` are processed as mount options. In either case, the same format requirements
as on plugin level also apply on volume level.

### Implementation
When setting up a volume process, i.e. if a `GetVolumeProcess()` function is
present, the driver calls it to obtain the basic command as well as volume
process and mount options specified on plugin level.
If either options are present, and a `SetVolumeProcessOptions()` function is
also present, volume level volume process and mount options are applied, and
finally that function is called to have it apply the resulting sets of options
to the command properly. If any options are present, but cannot be processed due
to a missing function, an error occurs.

White space character support for mount options is not (yet) defined, so for the
time being just do not simply use white space in any mount options, just as you
wouldn't when working with `mtab` directly.

# Helpful links
- [Process Management in Go](https://hackernoon.com/everything-you-need-to-know-about-managing-go-processes)
- [Preventing shallow copies with the `noCopy` struct](https://stackoverflow.com/questions/68183168/how-to-force-compiler-error-if-struct-shallow-copy)
- [Semantic Versioning 2.0.0](https://semver.org/)
- [W3Schools Go Tutorial](https://www.w3schools.com/go/go_syntax.php)
- [Go Constructors Tutorial](https://tutorialedge.net/golang/go-constructors-tutorial/)
- [Go Wiki: Well-known struct tags](https://go.dev/wiki/Well-known-struct-tags) (e.g. [`json`](https://pkg.go.dev/encoding/json#Marshal))
- [Comprehensive Guide to Type Assertion in Go](https://medium.com/@jamal.kaksouri/mastering-type-assertion-in-go-a-comprehensive-guide-216864b4ea4d)
- [Implementing (i.e. faking) Enums in Go](https://builtin.com/software-engineering-perspectives/golang-enum)
- [Demystifying Synchronous and Asynchronous Programming in Go](https://pandazblog.hashnode.dev/synchronous-vs-asynchronous-programming-in-golang)
- [Guide to Graceful Shutdown in Go](https://medium.com/@karthianandhanit/a-guide-to-graceful-shutdown-in-go-with-goroutines-and-context-1ebe3654cac8)
- [How to handle errors in Goroutines](https://medium.com/@rayato159/how-to-handle-errors-in-goroutines-0cced153551a)
- [Docker Engine managed plugin system documentation](https://docs.docker.com/engine/extend/)
- [Docker Plugins: How to write your own](https://www.inovex.de/de/blog/docker-plugins/)
