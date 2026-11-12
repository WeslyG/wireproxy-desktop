//go:build !windows

package main

import "os/exec"

func hideWorkerWindow(_ *exec.Cmd) {}
