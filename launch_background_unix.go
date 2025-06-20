//go:build !windows

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
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	err = cmd.Start()
	if err != nil {
		return err
	}

	pid := cmd.Process.Pid
	err = os.WriteFile("cron-go.pid", []byte(fmt.Sprintf("%d", pid)), 0644)
	if err != nil {
		fmt.Printf("Failed to write PID file: %v\n", err)
	}
	fmt.Printf("Started in background. PID: %d\n", pid)
	return nil
}
