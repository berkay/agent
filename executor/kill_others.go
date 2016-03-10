// +build !windows

package executor

import (
	"os/exec"
	"syscall"

	"github.com/neptuneio/agent/logging"
)

func KillCommand(cmd *exec.Cmd) {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, 15)
	} else {
		logging.Error("Could not get process group id from the command.", nil)
	}
}

func SetPGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
