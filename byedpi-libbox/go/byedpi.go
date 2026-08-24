// Package byedpi embeds the ciadpi DPI-desync local SOCKS proxy (ByeDPI)
// directly into libbox, so Apple-platform (iOS/macOS/tvOS) clients built from
// this source tree can detour eligible outbound TCP connections through it.
//
// It is a Go port of the Android SFA-ByeDPI integration:
//   - ByeDpiManager.kt       -> lifecycle below
//   - ByeDpiConfigInjector.kt -> inject.go
//
// Enable per profile by adding to the config JSON:
//
//	"experimental": {
//	  "byedpi": {
//	    "enabled": true,
//	    "listen_port": 1080,
//	    "command_line": "-Ku -a1 -An -o1 -At,r,s -d1"
//	  }
//	}
package byedpi

/*
#cgo darwin CFLAGS: -Dmain=ciadpi_main -O2
#cgo linux CFLAGS: -Dmain=ciadpi_main -O2
#include <stdlib.h>
#include "byedpi_shim.h"
*/
import "C"

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	host              = "127.0.0.1"
	portCheckTimeout  = 500 * time.Millisecond
	portReadyTimeout  = 5 * time.Second
	portClosedTimeout = 5 * time.Second
	retryInterval     = 100 * time.Millisecond

	defaultPort        = 1080
	defaultCommandLine = "-Ku -a1 -An -o1 -At,r,s -d1"
)

var (
	mu          sync.Mutex
	running     bool
	exited      chan struct{} // closed when the ciadpi thread returns
	currentArgs []string
	currentPort int
)

// Settings mirrors the experimental.byedpi profile block.
type Settings struct {
	Enabled     bool   `json:"enabled"`
	ListenPort  int    `json:"listen_port,omitempty"`
	CommandLine string `json:"command_line,omitempty"`
}

func parseShellArgs(input string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte('"')

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch == '"' || ch == '\'':
			if inQuotes && ch == quoteChar {
				inQuotes = false
			} else if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else {
				current.WriteByte(ch)
			}
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			if inQuotes {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// stripProtectArg reports whether a token is a protect-path flag, which is a
// no-op here: the OS never routes an extension's own sockets back into its
// own tunnel, so there is no loop to protect against (unlike Android).
func stripProtectArg(arg string) bool {
	return arg == "--protect-path" ||
		strings.HasPrefix(arg, "--protect-path=") ||
		arg == "-P" ||
		(strings.HasPrefix(arg, "-P") && len(arg) > 2)
}

func buildArguments(commandLine string, listenPort int) []string {
	// Defaults come first; ciadpi's option parser lets later duplicates win,
	// so user-supplied flags always take precedence (mirrors Android fork).
	args := []string{"ciadpi", "--ip", host}

	for _, arg := range parseShellArgs(commandLine) {
		if arg == "" || arg == "--help" || arg == "--version" || arg == "-h" || arg == "-v" {
			continue
		}
		if stripProtectArg(arg) {
			continue
		}
		args = append(args, arg)
	}

	hasPort := false
	for _, a := range args {
		if a == "--port" || strings.HasPrefix(a, "--port=") || strings.HasPrefix(a, "-p") {
			hasPort = true
			break
		}
	}
	if !hasPort {
		args = append(args, "--port", strconv.Itoa(listenPort))
	}
	return args
}

func extractPort(args []string) int {
	for i, arg := range args {
		switch {
		case arg == "--port" && i+1 < len(args):
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				return p
			}
		case strings.HasPrefix(arg, "--port="):
			if p, err := strconv.Atoi(strings.TrimPrefix(arg, "--port=")); err == nil {
				return p
			}
		case strings.HasPrefix(arg, "-p") && len(arg) > 2:
			if p, err := strconv.Atoi(arg[2:]); err == nil {
				return p
			}
		}
	}
	return defaultPort
}

func waitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp",
			net.JoinHostPort(host, strconv.Itoa(port)), portCheckTimeout)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(retryInterval)
	}
	return false
}

func waitForPortClosed(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp",
			net.JoinHostPort(host, strconv.Itoa(port)), portCheckTimeout)
		if err != nil {
			return true
		}
		conn.Close()
		time.Sleep(retryInterval)
	}
	return false
}

// checkExitedLocked reconciles state if the ciadpi thread self-exited.
// mu must be held.
func checkExitedLocked() {
	if running && exited != nil {
		select {
		case <-exited:
			running = false
			currentArgs = nil
			currentPort = 0
		default:
		}
	}
}

func startLocked(commandLine string, listenPort int) error {
	checkExitedLocked()

	if running {
		stopLocked()
		waitForPortClosed(currentPort, portClosedTimeout)
	}

	if commandLine == "" {
		commandLine = defaultCommandLine
	}
	if listenPort <= 0 {
		listenPort = defaultPort
	}

	args := buildArguments(commandLine, listenPort)
	port := extractPort(args)

	argv := make([]*C.char, len(args))
	for i, a := range args {
		argv[i] = (*C.char)(unsafe.Pointer(&[]byte{0}[0]))
		argv[i] = C.CString(a)
	}
	argc := C.int(len(args))
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		code := C.bd_start(argc, &argv[0])
		fmt.Printf("[byedpi] proxy exited with code %d\n", code)
		for _, p := range argv {
			C.free(unsafe.Pointer(p))
		}
	}()

	if !waitForPort(port, portReadyTimeout) {
		C.bd_force_close()
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
		}
		return fmt.Errorf("byedpi: listen port %d not ready after %v", port, portReadyTimeout)
	}

	running = true
	exited = doneCh
	currentArgs = args
	currentPort = port
	fmt.Printf("[byedpi] started successfully on port %d\n", port)
	return nil
}

// stopLocked stops the proxy; mu must be held.
func stopLocked() {
	checkExitedLocked()
	if !running {
		currentArgs = nil
		currentPort = 0
		return
	}

	C.bd_stop()
	ch := exited
	if ch != nil {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			fmt.Printf("[byedpi] not stopped in time, force closing\n")
			C.bd_force_close()
			select {
			case <-ch:
			case <-time.After(time.Second):
			}
		}
	}
	running = false
	currentArgs = nil
	currentPort = 0
}

// RestartIfNeeded starts the proxy when enabled, restarting it when the
// effective arguments changed since the previous run (handles reloads).
// When disabled it makes sure any previous instance is stopped.
func RestartIfNeeded(settings Settings) error {
	if !settings.Enabled {
		mu.Lock()
		stopLocked()
		mu.Unlock()
		return nil
	}

	commandLine := settings.CommandLine
	if commandLine == "" {
		commandLine = defaultCommandLine
	}
	listenPort := settings.ListenPort
	if listenPort <= 0 {
		listenPort = defaultPort
	}
	effective := buildArguments(commandLine, listenPort)

	mu.Lock()
	defer mu.Unlock()

	checkExitedLocked()
	if running && equalStrings(effective, currentArgs) {
		return nil
	}
	return startLocked(commandLine, listenPort)
}

// Stop shuts down any running instance.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	stopLocked()
}

// Running reports whether the embedded proxy is up.
func Running() bool {
	mu.Lock()
	defer mu.Unlock()
	checkExitedLocked()
	return running
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
