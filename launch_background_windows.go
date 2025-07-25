//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"fmt"
)


func launchBackground(cfgPath []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, cfgPath...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	err = cmd.Start()
	if err != nil {
		return err
	}

	pid := cmd.Process.Pid
	fmt.Printf("Started in background. PID: %d\n", pid)
	return nil
}
