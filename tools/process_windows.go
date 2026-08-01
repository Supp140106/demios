//go:build windows

package tools

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func killProcessTree(pid int) {
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// processAlive reports whether a process with the given PID exists.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}
