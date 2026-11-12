package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

const tunnelEstablished = "Tunnel established"

type proxySession struct {
	mu                        sync.Mutex
	process                   *exec.Cmd
	input                     io.WriteCloser
	done                      chan struct{}
	notify                    func()
	profile, address, status  string
	ready, stopping, checking bool
	logs                      []string
	options                   settings
	protocol                  string
	checkCancel               context.CancelFunc
	checkGeneration           uint64
	checkResult               *socksCheckResult
}

type socksCheckResult struct {
	status string
	err    error
}

type sessionView struct {
	profile, address, status string
	active, ready, checking  bool
	logs                     []string
	protocol                 string
	checkResult              *socksCheckResult
}

func (v sessionView) tunnelReady() bool {
	return v.active && tunnelReady(v.status, v.ready)
}

func tunnelReady(status string, ready bool) bool {
	return ready && status == tunnelEstablished
}

func (s *proxySession) view(includeLogs bool) sessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := sessionView{profile: s.profile, address: s.address, status: s.status,
		active: s.process != nil, ready: s.ready, checking: s.checking, protocol: s.protocol, checkResult: s.checkResult}
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
	s.options = options
	s.protocol = ""
	s.profile, s.address, s.status = p.Name, options.bindAddress(), "Starting"
	s.logLocked("Starting " + p.Name)
	go s.monitor(cmd, input, output, workerConfig{Config: p.Config, settings: options})
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
				s.status = tunnelEstablished
			}
			s.logLocked("AmneziaWG / WireGuard handshake confirmed")
		case handshakeLost:
			if !s.stopping {
				s.status = "Tunnel unavailable"
				s.cancelCheckLocked()
				s.logLocked("AmneziaWG / WireGuard handshake expired")
			}
		default:
			if protocol, ok := strings.CutPrefix(line, protocolStatus); ok {
				s.protocol = protocol
				s.logLocked("Tunnel protocol: " + protocol)
			} else {
				s.logLocked(line)
			}
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
	s.cancelCheckLocked()
	s.ready, s.stopping = false, false
	close(s.done)
	s.mu.Unlock()
	s.notify()
}

func (s *proxySession) stop() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil && !s.stopping {
		s.ready, s.stopping, s.status = false, true, "Stopping"
		s.cancelCheckLocked()
		input, process, done := s.input, s.process.Process, s.done
		go input.Close()
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

func (s *proxySession) cancelCheckLocked() {
	s.checkGeneration++
	if s.checkCancel != nil {
		s.checkCancel()
		s.checkCancel = nil
	}
	s.checking = false
}

func (s *proxySession) check() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process == nil || !canCheckSOCKS(s.status, s.ready, s.checking) {
		return false
	}
	generation, options := s.checkGeneration, s.options
	s.checking = true
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	s.checkCancel = cancel
	s.logLocked("Checking HTTPS through SOCKS5")
	go func() {
		defer cancel()
		status, err := checkHTTPS(ctx, options)
		s.mu.Lock()
		if s.checkGeneration == generation {
			s.checking = false
			s.checkCancel = nil
			if !s.stopping {
				s.checkResult = &socksCheckResult{status: status, err: err}
				if err != nil {
					s.logLocked("SOCKS5 test failed: " + err.Error())
				} else {
					s.logLocked("SOCKS5 test passed: HTTP " + status)
				}
			}
		}
		s.mu.Unlock()
		s.notify()
	}()
	return true
}

func canCheckSOCKS(status string, ready, checking bool) bool {
	return tunnelReady(status, ready) && !checking
}

func checkHTTPS(ctx context.Context, options settings) (string, error) {
	var auth *proxy.Auth
	if options.Username != "" {
		auth = &proxy.Auth{User: options.Username, Password: options.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", options.clientAddress(), auth, proxy.Direct)
	if err != nil {
		return "", err
	}
	transport := &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return checkHTTPSEndpoints(ctx, client)
}

func checkHTTPSEndpoints(ctx context.Context, client *http.Client) (string, error) {
	var failures []error
	for _, endpoint := range []struct {
		url    string
		status int
	}{
		{"https://example.com/", http.StatusOK},
		{"https://www.gstatic.com/generate_204", http.StatusNoContent},
	} {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
		req, err := http.NewRequestWithContext(attempt, http.MethodGet, endpoint.url, nil)
		if err != nil {
			cancel()
			return "", err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == endpoint.status {
				cancel()
				return resp.Status, nil
			}
			err = fmt.Errorf("HTTP %s", resp.Status)
		}
		cancel()
		failures = append(failures, fmt.Errorf("%s: %w", req.URL.Host, err))
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", errors.Join(failures...)
}
