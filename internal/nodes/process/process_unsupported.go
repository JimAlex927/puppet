//go:build !windows && !linux

package process

import (
	"fmt"
	"os"
	"os/exec"

	"puppet/internal/node"
)

func configureProcessCommand(cmd *exec.Cmd, showWindow bool) {}

func launchProcess(executable string, args []string, workdir string, showWindow bool, stdout, stderr *os.File) (int, error) {
	return 0, fmt.Errorf("process node is not supported on this OS")
}

func processSupported() bool { return false }

func queryProcessInfo(ctx *node.NodeContext, pid int) (processInfo, error) {
	return processInfo{}, fmt.Errorf("process node is not supported on this OS")
}

func pidsByProcessName(ctx *node.NodeContext, name string) ([]int, error) {
	return nil, fmt.Errorf("process node is not supported on this OS")
}

func pidsByPort(ctx *node.NodeContext, port int) ([]int, error) {
	return nil, fmt.Errorf("process node is not supported on this OS")
}

func killPID(ctx *node.NodeContext, pid int, force bool) error {
	return fmt.Errorf("process node is not supported on this OS")
}

func killManagedPID(ctx *node.NodeContext, pid int, force bool) error {
	return fmt.Errorf("process node is not supported on this OS")
}
