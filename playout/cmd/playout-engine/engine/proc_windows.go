//go:build !cli && windows

package engine

import "os/exec"

func setProcAttr(cmd *exec.Cmd) {}
