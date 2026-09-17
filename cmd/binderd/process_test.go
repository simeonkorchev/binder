package main_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	// sessionSecret signs the sessions these specs mint. It is 32 bytes because
	// session.New refuses anything shorter, and it is a test value: the server
	// under test is reachable only on the loopback port it was just given.
	sessionSecret = "smoke-test-session-secret-32-byte"
	// startTimeout is how long a spec waits for the process to announce its
	// address. A cold start here is a process exec and one database connection.
	startTimeout = 30 * time.Second
	// stopTimeout is how long a spec waits for the process to drain and exit
	// after SIGTERM. Longer than the server's own drain timeout, so a spec that
	// times out means the shutdown path is stuck rather than merely slow.
	stopTimeout = 30 * time.Second
	// pollInterval paces both waits.
	pollInterval = 50 * time.Millisecond
)

// listeningAddress pulls the bound address out of the line the server logs once
// it is listening. PORT=0 means the operating system chooses the port, so this
// is how a spec learns which one it got.
var listeningAddress = regexp.MustCompile(`address=(\S+)`)

// logBuffer collects a child process's stderr. The process's output is written
// by the os/exec copier goroutine while a spec reads it, so the buffer is
// mutex-guarded: -race is on, and an unguarded bytes.Buffer here is a genuine
// data race rather than a theoretical one.
type logBuffer struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}

// serverProcess is a running binderd, and the address it answers on.
type serverProcess struct {
	cmd     *exec.Cmd
	logs    *logBuffer
	baseURL string
}

// defaultEnv is the configuration a server starts with unless a spec changes it.
// It is a fresh map per call so that a spec can delete a key from it without
// affecting the next one.
func defaultEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":       databaseURL,
		"SESSION_JWT_SECRET": sessionSecret,
		"APPLE_CLIENT_IDS":   "com.binder.test",
		"GOOGLE_CLIENT_IDS":  "binder-test.apps.googleusercontent.com",
		// The operating system picks a free port. Two suites, or two agents, can
		// then run this at once without agreeing on a number first.
		"PORT": "0",
	}
}

// launch starts the server binary with env and nothing else on its environment,
// so a value the developer happens to have exported cannot stand in for one the
// spec meant to set — or, for the missing-configuration spec, meant to omit.
func launch(env map[string]string) *serverProcess {
	GinkgoHelper()

	logs := &logBuffer{}
	cmd := exec.CommandContext(context.Background(), binaryPath)
	cmd.Env = environ(env)
	cmd.Stderr = logs
	cmd.Stdout = logs

	Expect(cmd.Start()).To(Succeed())
	return &serverProcess{cmd: cmd, logs: logs, baseURL: ""}
}

// start launches a server and waits until it is listening, returning it with the
// base URL of the port it bound.
func start(env map[string]string) *serverProcess {
	GinkgoHelper()

	server := launch(env)
	var address string
	Eventually(func() string {
		address = boundAddress(server.logs.String())
		return address
	}, startTimeout, pollInterval).ShouldNot(BeEmpty(),
		"server never logged a listening address:\n%s", server.logs.String())

	// The logged address is whatever the listener bound, which for an empty host
	// is the all-interfaces form ([::]:41234). Only the port is portable enough
	// to dial back on.
	server.baseURL = "http://127.0.0.1:" + address[strings.LastIndex(address, ":")+1:]
	return server
}

// stop asks the server to shut down the way a deploy does, and fails the spec if
// it does not exit cleanly.
func (s *serverProcess) stop() {
	GinkgoHelper()

	Expect(s.cmd.Process.Signal(syscall.SIGTERM)).To(Succeed())

	exited := make(chan error, 1) // buffered: the waiter must never block on a spec that already failed.
	go func() { exited <- s.cmd.Wait() }()

	var waitErr error
	Eventually(exited, stopTimeout, pollInterval).Should(Receive(&waitErr),
		"server did not exit on SIGTERM:\n%s", s.logs.String())
	Expect(waitErr).NotTo(HaveOccurred(), "server exited non-zero on SIGTERM:\n%s", s.logs.String())
}

// do sends one request to the running server. token, when not empty, is sent as
// the session bearer; body, when not empty, is sent as JSON.
func (s *serverProcess) do(method, path, token, body string) *http.Response {
	GinkgoHelper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, s.baseURL+path, reader)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "%s %s over the socket", method, path)
	DeferCleanup(func() { Expect(resp.Body.Close()).To(Succeed()) })

	return resp
}

// readBody drains a response, so a spec can report what the server actually said
// when an assertion about it fails.
func readBody(resp *http.Response) string {
	GinkgoHelper()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	return string(body)
}

// environ renders a map as the KEY=VALUE slice os/exec wants.
func environ(env map[string]string) []string {
	rendered := make([]string, 0, len(env))
	for key, value := range env {
		rendered = append(rendered, fmt.Sprintf("%s=%s", key, value))
	}
	return rendered
}

// boundAddress returns the address in the server's listening line, or "" when it
// has not logged one yet.
func boundAddress(logs string) string {
	match := listeningAddress.FindStringSubmatch(logs)
	if match == nil {
		return ""
	}
	return match[1]
}
