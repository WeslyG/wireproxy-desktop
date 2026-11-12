package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type proxySession struct {
	mu                        sync.Mutex
	process                   *exec.Cmd
	input                     io.WriteCloser
	done                      chan struct{}
	notify                    func()
	profile, address, status  string
	ready, stopping, checking bool
	logs                      []string
}

type sessionView struct {
	profile, address, status string
	active, ready, checking  bool
	logs                     []string
}

func (s *proxySession) view(includeLogs bool) sessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := sessionView{s.profile, s.address, s.status, s.process != nil, s.ready, s.checking, nil}
	if includeLogs {
		v.logs = append([]string(nil), s.logs...)
	}
	return v
}

func (s *proxySession) logLocked(message string) {
	const limit = 500
	if len(s.logs) == limit {
		copy(s.logs, s.logs[1:])
		s.logs = s.logs[:limit-1]
	}
	s.logs = append(s.logs, time.Now().Format("15:04:05")+"  "+message)
}

func (s *proxySession) clearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = nil
}

func (s *proxySession) start(p profile, options settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil {
		return fmt.Errorf("disconnect the running proxy first")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, workerFlag)
	hideWorkerWindow(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	s.process, s.input, s.done = cmd, input, make(chan struct{})
	s.profile, s.address, s.status = p.Name, proxyAddress(options.Port), "Starting"
	s.logLocked("Starting " + p.Name)
	go s.monitor(cmd, input, output, workerConfig{p.Config, options.Port, options.Verbose})
	return nil
}

func (s *proxySession) monitor(cmd *exec.Cmd, input io.WriteCloser, output io.ReadCloser, cfg workerConfig) {
	if err := json.NewEncoder(input).Encode(cfg); err != nil {
		_ = input.Close()
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		s.mu.Lock()
		switch line := scanner.Text(); line {
		case listenerReady:
			if !s.stopping {
				s.ready = true
				s.status = "SOCKS5 listening"
			}
			s.logLocked("SOCKS5 listening on " + s.address)
		case handshakeReady:
			if !s.stopping {
				s.status = "Tunnel established"
			}
			s.logLocked("AmneziaWG / WireGuard handshake confirmed")
		default:
			s.logLocked(line)
		}
		s.mu.Unlock()
		s.notify()
	}
	if scanner.Err() != nil {
		_ = cmd.Process.Kill()
	}
	err := cmd.Wait()
	s.mu.Lock()
	s.status = "Disconnected"
	if err != nil && !s.stopping {
		s.status = "Proxy failed"
		s.logLocked("Proxy stopped unexpectedly. Check the preceding log entries.")
	} else {
		s.logLocked("Proxy stopped")
	}
	s.process, s.input = nil, nil
	s.ready, s.stopping, s.checking = false, false, false
	close(s.done)
	s.mu.Unlock()
	s.notify()
}

func (s *proxySession) stop() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil && !s.stopping {
		s.ready, s.stopping, s.status = false, true, "Stopping"
		_ = s.input.Close()
		process, done := s.process.Process, s.done
		go func() {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = process.Kill()
			}
		}()
	}
	return s.done
}

func (s *proxySession) check() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready || s.checking {
		return
	}
	process, address := s.process, s.address
	s.checking = true
	s.logLocked("Checking HTTPS through SOCKS5")
	go func() {
		message := checkHTTPS(address)
		s.mu.Lock()
		if s.process == process {
			s.checking = false
			s.logLocked(message)
		}
		s.mu.Unlock()
		s.notify()
	}()
}

func checkHTTPS(address string) string {
	dialer, err := proxy.SOCKS5("tcp", address, nil, proxy.Direct)
	if err != nil {
		return "Could not initialize the SOCKS5 check"
	}
	transport := &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/", nil)
	if err != nil {
		return "Could not initialize the HTTPS check"
	}
	resp, err := client.Do(req)
	if err != nil {
		return "SOCKS5 check failed: " + err.Error()
	}
	_ = resp.Body.Close()
	return "SOCKS5 check returned HTTP " + resp.Status
}
