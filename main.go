package main

import (
	"fmt"
	"os"

	"github.com/WeslyG/wireproxy-desktop/internal/desktop"
	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
	"github.com/WeslyG/wireproxy-desktop/internal/updater"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == tunnel.ProbeFlag {
		if err := tunnel.RunProbeWorker(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println(version)
		return
	}
	if len(args) > 0 && args[0] == updater.ApplyFlag {
		if err := updater.RunHelper(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == tunnel.WorkerFlag {
		if err := tunnel.RunWorker(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	desktop.Run(args, version)
}

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64,arm64 --manifest gui --icon assets/favicon.ico --out icon
