package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/thorbenw/example-docker-volume-plugin/proc"
	"github.com/thorbenw/example-docker-volume-plugin/utils"
	"gotest.tools/assert"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))

func Test_exampleDriver(t *testing.T) {
	proc.Logger = logger

	volumeFolder := t.TempDir()
	driver, _ := exampleDriver_New(volumeFolder, *logger)
	if runBinary, err := exec.LookPath("inotifywatch"); err != nil {
		t.Errorf("inotify-tools need to be installed (%s): sudo apt install inotify-tools", err.Error())
	} else {
		driver.VolumeProcess = func(path string) *exec.Cmd {
			return exec.Command(runBinary, path)
		}
	}

	if driver.Capabilities() == nil {
		t.Error("Failed to gather driver capabilities.")
	}

	volumeNames := []string{"test_volume1", "test_volume2"}
	mountIds := []string{"test_id1", "test_id2"}

	testCreate := func(volumeName string) {
		if _, err := driver.Get(&volume.GetRequest{Name: volumeName}); err == nil {
			t.Fatalf("Volume [%s] hasn't been expected to exist.", volumeName)
		}

		if err := driver.Create(&volume.CreateRequest{Name: volumeName}); err != nil {
			t.Errorf("Creating volume [%s] failed (%s).", volumeName, err.Error())
		}
		if err := driver.Create(&volume.CreateRequest{Name: volumeName}); err == nil {
			t.Errorf("Creating volume [%s] twice.", volumeName)
		}
	}

	testRemove := func(volumeName string) {
		if err := driver.Remove(&volume.RemoveRequest{Name: volumeName}); err != nil {
			t.Errorf("Removing volume [%s] failed (%s).", volumeName, err.Error())
		}
		if err := driver.Remove(&volume.RemoveRequest{Name: volumeName}); err == nil {
			t.Errorf("Removing volume [%s] twice.", volumeName)
		}
	}

	testMount := func(volumeName string, mountId string) {
		if res, err := driver.Mount(&volume.MountRequest{Name: volumeName, ID: mountId}); err != nil {
			t.Errorf("Mounting volume [%s] for id [%s] failed (%s).", volumeName, mountId, err.Error())
		} else {
			assert.Assert(t, res.Mountpoint == filepath.Join(driver.PropagatedMount, utils.SHA256StringToString(volumeName)))
		}
		if res, err := driver.Mount(&volume.MountRequest{Name: volumeName, ID: mountId}); err != nil {
			t.Errorf("Incrementing volume [%s] for id [%s] failed (%s).", volumeName, mountId, err.Error())
		} else {
			assert.Assert(t, res.Mountpoint == filepath.Join(driver.PropagatedMount, utils.SHA256StringToString(volumeName)))
		}
	}

	testUnmount := func(volumeName string, mountId string) {
		if err := driver.Unmount(&volume.UnmountRequest{Name: volumeName, ID: mountId}); err != nil {
			t.Errorf("Decrementing volume [%s] for id [%s] failed (%s).", volumeName, mountId, err.Error())
		}
		if err := driver.Unmount(&volume.UnmountRequest{Name: volumeName, ID: mountId}); err != nil {
			t.Errorf("Unmounting volume [%s] for id [%s] failed (%s).", volumeName, mountId, err.Error())
		}
		if err := driver.Unmount(&volume.UnmountRequest{Name: volumeName, ID: mountId}); err != nil {
			t.Errorf("Submounting volume [%s] for id [%s] failed (%s).", volumeName, mountId, err.Error())
		}
	}

	for _, volumeName := range volumeNames {
		testCreate(volumeName)
		for _, mountId := range mountIds {
			testMount(volumeName, mountId)
		}
	}

	if json, err := os.ReadFile(driver.ControlFile.Name()); err != nil {
		t.Fatal(err)
	} else {
		fmt.Printf("Current JSON from control file:\n%s\n", string(json))
	}

	driver, err := exampleDriver_New(volumeFolder, *logger)
	if err != nil {
		t.Fatal(err)
	}

	for _, volumeName := range volumeNames {
		for _, mountId := range mountIds {
			testUnmount(volumeName, mountId)
		}
		testRemove(volumeName)
	}
}

func Test_exampleDriver_New(t *testing.T) {
	t.Parallel()

	if driver, err := exampleDriver_New("", *logger); err == nil {
		t.Errorf("Instanciation of new %T succeeded unexpectedly.", driver)
	} else {
		logger.Debug(err.Error())
	}

	if driver, err := exampleDriver_New(DEFAULT_PLUGIN_SOCK_DIR, *logger); err == nil {
		t.Errorf("Instanciation of new %T succeeded unexpectedly.", driver)
	} else {
		logger.Debug(err.Error())
	}

	propagatedMount := t.TempDir()

	testFile := filepath.Join(propagatedMount, utils.SHA256StringToString("Test_exampleDriver_New"))
	if err := os.WriteFile(testFile, []byte{}, os.ModeExclusive); err != nil {
		t.Fatal(err)
	}
	if driver, err := exampleDriver_New(testFile, *logger); err == nil {
		t.Errorf("Instanciation of new %T succeeded unexpectedly.", driver)
	} else {
		logger.Debug(err.Error())
	}
	if driver, err := exampleDriver_New(filepath.Dir(DEFAULT_PLUGIN_SOCK_DIR), *logger); err == nil {
		t.Errorf("Instanciation of new %T succeeded unexpectedly.", driver)
	} else {
		logger.Debug(err.Error())
	}

	testFile = filepath.Join(propagatedMount, DefaultControlFileName)
	if err := os.WriteFile(testFile, []byte(testFile), DefaultControlFileMode); err != nil {
		t.Fatal(err)
	}
	if driver, err := exampleDriver_New(propagatedMount, *logger); err == nil {
		t.Errorf("Instanciation of new %T succeeded unexpectedly.", driver)
	} else {
		logger.Debug(err.Error())
	}
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}

	if driver, err := exampleDriver_New(propagatedMount, *logger); err != nil {
		t.Error(err.Error())
	} else {
		assert.Assert(t, driver.Volumes != nil)
		assert.Assert(t, len(driver.Volumes) < 1)
	}
}

func Test_exampleDriver_Get(t *testing.T) {
	t.Parallel()

	driver, _ := exampleDriver_New(t.TempDir(), *logger)
	volumeName := utils.SHA256StringToString("Test_exampleDriver_Get")
	if _, err := driver.Get(&volume.GetRequest{Name: volumeName}); err == nil {
		t.Fatalf("Volume [%s] hasn't been expected to exist.", volumeName)
	}

	if err := driver.Create(&volume.CreateRequest{Name: volumeName}); err != nil {
		t.Errorf("Creating volume [%s] failed (%s).", volumeName, err.Error())
	} else {
		if res, err := driver.Get(&volume.GetRequest{Name: volumeName}); err != nil {
			t.Errorf("Volume [%s] has been expected to exist.", volumeName)
		} else {
			if time_res, err := time.Parse(time.RFC3339, res.Volume.CreatedAt); err != nil {
				t.Errorf("Parsing %T.CreatedAt (%s) failed.", res, res.Volume.CreatedAt)
			} else {
				time_now := time.Now()
				assert.Assert(t, time_res.Unix() <= time_now.Unix(), "CreatedAt [%s] is expected to be in the past.", res.Volume.CreatedAt)
			}
			assert.Assert(t, res.Volume.Mountpoint == filepath.Join(driver.PropagatedMount, utils.SHA256StringToString(res.Volume.Name)))
		}
	}
}

func Test_exampleDriver_Path(t *testing.T) {
	t.Parallel()

	driver, _ := exampleDriver_New(t.TempDir(), *logger)
	volumeName := utils.SHA256StringToString("Test_exampleDriver_Path")
	if _, err := driver.Path(&volume.PathRequest{Name: volumeName}); err == nil {
		t.Fatalf("Volume [%s] hasn't been expected to exist.", volumeName)
	}

	if err := driver.Create(&volume.CreateRequest{Name: volumeName}); err != nil {
		t.Errorf("Creating volume [%s] failed (%s).", volumeName, err.Error())
	} else {
		if res, err := driver.Path(&volume.PathRequest{Name: volumeName}); err != nil {
			t.Errorf("Volume [%s] has been expected to exist.", volumeName)
		} else {
			assert.Assert(t, len(strings.TrimSpace(res.Mountpoint)) > 0, "%T res.MountPoint [%s] is empty.", res.Mountpoint, res.Mountpoint)
			assert.Assert(t, res.Mountpoint == filepath.Join(driver.PropagatedMount, utils.SHA256StringToString(volumeName)))
		}
	}
}

func Test_exampleDriver_List(t *testing.T) {
	t.Parallel()

	driver, _ := exampleDriver_New(t.TempDir(), *logger)
	if res, err := driver.List(); err != nil {
		t.Errorf("%s", err.Error())
	} else {
		assert.Assert(t, res != nil)
		assert.Assert(t, res.Volumes != nil)
		assert.Assert(t, len(res.Volumes) < 1)
	}

	volumeName := utils.SHA256StringToString("Test_exampleDriver_List")
	if err := driver.Create(&volume.CreateRequest{Name: volumeName}); err != nil {
		t.Fatalf("Creating volume [%s] failed (%s).", volumeName, err.Error())
	}

	if res, err := driver.List(); err != nil {
		t.Errorf("%s", err.Error())
	} else {
		assert.Assert(t, res != nil)
		assert.Assert(t, res.Volumes != nil)
		assert.Assert(t, len(res.Volumes) == 1)
		for _, vol := range res.Volumes {
			assert.Assert(t, vol.Mountpoint == filepath.Join(driver.PropagatedMount, utils.SHA256StringToString(vol.Name)))
		}
	}
}

func Test_exampleDriver_Load(t *testing.T) {
	testFileName := filepath.Join(t.TempDir(), DefaultControlFileName)

	testFile, err := os.OpenFile(testFileName, os.O_RDWR|os.O_CREATE, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}
	if volumes, err := exampleDriver_Load(*testFile); err == nil {
		t.Errorf("Loading invalid file to a %T succeeded unexpectedly.", volumes)
	} else {
		logger.Debug(err.Error())
	}

	testFile, err = os.OpenFile(testFileName, os.O_WRONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if volumes, err := exampleDriver_Load(*testFile); err == nil {
		t.Errorf("Loading invalid file to a %T succeeded unexpectedly.", volumes)
	} else {
		logger.Debug(err.Error())
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}

	testFile, err = os.OpenFile(testFileName, os.O_RDONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if volumes, err := exampleDriver_Load(*testFile); err != nil {
		t.Error(err)
	} else {
		assert.Assert(t, len(volumes) < 1)
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(testFileName, []byte("{}"), DefaultControlFileMode); err != nil {
		t.Fatal(err)
	}
	testFile, err = os.OpenFile(testFileName, os.O_RDONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if volumes, err := exampleDriver_Load(*testFile); err != nil {
		t.Error(err)
	} else {
		assert.Assert(t, len(volumes) < 1)
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(testFileName, []byte(testFileName), DefaultControlFileMode); err != nil {
		t.Fatal(err)
	}
	testFile, err = os.OpenFile(testFileName, os.O_RDONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if volumes, err := exampleDriver_Load(*testFile); err == nil {
		t.Errorf("Loading invalid file to a %T succeeded unexpectedly.", volumes)
	} else {
		logger.Debug(err.Error())
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}
}

func Test_exampleDriver_Save(t *testing.T) {
	testFileName := filepath.Join(t.TempDir(), DefaultControlFileName)
	volumes := map[string]exampleDriverVolume{}

	testFile, err := os.OpenFile(testFileName, os.O_RDWR|os.O_CREATE, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := exampleDriver_Save(*testFile, volumes); err == nil {
		t.Errorf("Saving invalid %T succeeded unexpectedly (expected file to be already closed).", testFile)
	} else {
		logger.Debug(err.Error())
	}

	testFile, err = os.OpenFile(testFileName, os.O_RDONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if err := exampleDriver_Save(*testFile, volumes); err == nil {
		t.Errorf("Saving invalid %T succeeded unexpectedly (expected fail due to file being opened read-only).", testFile)
	} else {
		logger.Debug(err.Error())
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}

	testFile, err = os.OpenFile(testFileName, os.O_WRONLY, DefaultControlFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if err := exampleDriver_Save(*testFile, volumes); err != nil {
		t.Error(err)
	}
	if err := testFile.Close(); err != nil {
		t.Fatal(err)
	}
}
