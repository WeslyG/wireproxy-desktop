package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const probeWorkerFlag = "--probe-worker"

type probeOutcome struct {
	Error string
}

func runProbeWorker(input io.Reader, output io.Writer) error {
	runtime.GOMAXPROCS(2)
	var config string
	if err := json.NewDecoder(io.LimitReader(input, maxConfigLength*6+16)).Decode(&config); err != nil {
		return err
	}
	var result probeOutcome
	if err := probeProfile(config); err != nil {
		result.Error = err.Error()
	}
	return json.NewEncoder(output).Encode(result)
}

func probeInProcess(ctx context.Context, config string) error {
	ctx, cancel := context.WithTimeout(ctx, 16*time.Second)
	defer cancel()
	if len(config) == 0 || len(config) > maxConfigLength {
		return errors.New("Enter a configuration to check.")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	input, err := json.Marshal(config)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executable, probeWorkerFlag)
	hideWorkerWindow(command)
	command.Stdin = strings.NewReader(string(input))
	data, err := command.Output()
	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return errors.New("Connection test timed out.")
		case context.Canceled:
			return ctx.Err()
		}
		return errors.New("Could not run the connection test.")
	}
	var result probeOutcome
	if err := json.Unmarshal(data, &result); err != nil {
		return errors.New("Could not read the connection test result.")
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	return nil
}
