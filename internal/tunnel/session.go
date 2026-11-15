package tunnel

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

	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/process"
)

type tunnelStatus string

const (
	sessionStarting     tunnelStatus = "Starting"
	Listening           tunnelStatus = "SOCKS5 listening"
	Established         tunnelStatus = "Tunnel established"
	Unavailable         tunnelStatus = "Tunnel unavailable"
	Stopping            tunnelStatus = "Stopping"
	sessionDisconnected tunnelStatus = "Disconnected"
	Failed              tunnelStatus = "Proxy failed"
)

type Session struct {
	mu                        sync.Mutex
	process                   *exec.Cmd
	input                     io.WriteCloser
	done                      chan struct{}
	Notify                    func()
	profile, address          string
	status                    tunnelStatus
	ready, stopping, checking bool
	logs                      []string
	options                   config.Settings
	protocol                  string
	checkCancel               context.CancelFunc
	checkGeneration           uint64
	checkResult               *CheckResult
}

type CheckResult struct {
	Status string
	Err    error
}

type View struct {
	Profile, Address        string
	Status                  tunnelStatus
	Active, Ready, Checking bool
	Logs                    []string
	Protocol                string
	CheckResult             *CheckResult
}

func (v View) TunnelReady() bool {
	return v.Active && tunnelReady(v.Status, v.Ready)
}

func tunnelReady(status tunnelStatus, ready bool) bool {
	return ready && status == Established
}

func (s *Session) View(includeLogs bool) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := View{Profile: s.profile, Address: s.address, Status: s.status,
		Active: s.process != nil, Ready: s.ready, Checking: s.checking, Protocol: s.protocol, CheckResult: s.checkResult}
	if includeLogs {
		v.Logs = append([]string(nil), s.logs...)
	}
	return v
}

func (s *Session) logLocked(message string) {
	const limit = 500
	if len(s.logs) == limit {
		copy(s.logs, s.logs[1:])
		s.logs = s.logs[:limit-1]
	}
	s.logs = append(s.logs, time.Now().Format("15:04:05")+"  "+message)
}

func (s *Session) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = nil
}

func (s *Session) Start(p config.Profile, options config.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil {
		return fmt.Errorf("disconnect the running proxy first")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, WorkerFlag)
	process.HideWindow(cmd)
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
	s.profile, s.address, s.status = p.Name, options.BindAddress(), sessionStarting
	s.logLocked("Starting " + p.Name)
	go s.monitor(cmd, input, output, workerConfig{Config: p.Config, Settings: options})
	return nil
}

func (s *Session) monitor(cmd *exec.Cmd, input io.WriteCloser, output io.ReadCloser, cfg workerConfig) {
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
				s.status = Listening
			}
			s.logLocked("SOCKS5 listening on " + s.address)
		case handshakeReady:
			if !s.stopping {
				s.status = Established
			}
			s.logLocked("AmneziaWG / WireGuard handshake confirmed")
		case handshakeLost:
			if !s.stopping {
				s.status = Unavailable
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
		s.Notify()
	}
	if scanner.Err() != nil {
		_ = cmd.Process.Kill()
	}
	err := cmd.Wait()
	s.mu.Lock()
	s.status = sessionDisconnected
	if err != nil && !s.stopping {
		s.status = Failed
		s.logLocked("Proxy stopped unexpectedly. Check the preceding log entries.")
	} else {
		s.logLocked("Proxy stopped")
	}
	s.process, s.input = nil, nil
	s.cancelCheckLocked()
	s.ready, s.stopping = false, false
	close(s.done)
	s.mu.Unlock()
	s.Notify()
}

func (s *Session) Stop() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil && !s.stopping {
		s.ready, s.stopping, s.status = false, true, Stopping
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

func (s *Session) cancelCheckLocked() {
	s.checkGeneration++
	if s.checkCancel != nil {
		s.checkCancel()
		s.checkCancel = nil
	}
	s.checking = false
}

func (s *Session) Check() bool {
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
				s.checkResult = &CheckResult{Status: status, Err: err}
				if err != nil {
					s.logLocked("SOCKS5 test failed: " + err.Error())
				} else {
					s.logLocked("SOCKS5 test passed: HTTP " + status)
				}
			}
		}
		s.mu.Unlock()
		s.Notify()
	}()
	return true
}

func canCheckSOCKS(status tunnelStatus, ready, checking bool) bool {
	return tunnelReady(status, ready) && !checking
}

func checkHTTPS(ctx context.Context, options config.Settings) (string, error) {
	var auth *proxy.Auth
	if options.Username != "" {
		auth = &proxy.Auth{User: options.Username, Password: options.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", options.ClientAddress(), auth, proxy.Direct)
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
