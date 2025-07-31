package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func Test_entryPoint(t *testing.T) {

	testEntryPoint := func(args []string, expected int) {
		if actual := entryPoint("TestEntryPoint", args); actual != expected {
			t.Errorf("Entry point returned code %d insted of %d when called with arguments %s.", actual, expected, args)
		}
	}

	testFold := t.TempDir()
	testFile := filepath.Join(testFold, "testFile")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatal(err.Error())
	}

	t.Setenv("LOG_LEVEL", "info")
	testEntryPoint([]string{"--test"}, EXIT_CODE_USAGE)
	testEntryPoint([]string{"--help"}, EXIT_CODE_HELP)
	testEntryPoint([]string{"--help", "--version"}, EXIT_CODE_HELP)
	testEntryPoint([]string{"--version"}, EXIT_CODE_OK)
	testEntryPoint([]string{"--version", "--log-level=test"}, EXIT_CODE_OK)
	testEntryPoint([]string{"--build-info"}, EXIT_CODE_OK)
	testEntryPoint([]string{"--log-level=test", "--volume-process-recovery-mode=test", "--volume-process-recovery-max-per-min=0"}, EXIT_CODE_PARAM)
	testEntryPoint([]string{"--log-level=test", "--volume-process-recovery-mode=restart", "--volume-process-recovery-max-per-min=0"}, EXIT_CODE_PARAM)
	testEntryPoint([]string{"--log-level=debug", fmt.Sprintf("--propagated-mount=%s", testFile)}, EXIT_CODE_PARAM)

	t.Setenv("LOG_SOURCE", "true")
	t.Setenv("VOLUME_PROCESS_RECOVERY_MODE", "restart")
	t.Setenv("VOLUME_PROCESS_RECOVERY_MAX_PER_MIN", "1")
	testEntryPoint([]string{"--log-level=debug", fmt.Sprintf("--propagated-mount=%s", testFold)}, EXIT_CODE_ERROR)

	t.Setenv("DEFAULT_MOUNT_OPTIONS", "key1=val1")
	testEntryPoint([]string{"--log-level=debug", "-o=key2=val2", fmt.Sprintf("--propagated-mount=%s", testFold)}, EXIT_CODE_ERROR)
}
