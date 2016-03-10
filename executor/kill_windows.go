package executor

import (
	"os/exec"

	"github.com/neptuneio/agent/logging"
)

func KillCommand(cmd *exec.Cmd) {
	err := cmd.Process.Kill()
	if err != nil {
		logging.Error("Could not kill the command after timeout.", nil)
	}
}

func SetPGroup(cmd *exec.Cmd) {
	// Nothing to do.
}
