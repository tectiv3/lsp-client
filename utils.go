package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime/debug"

	lsp "github.com/tectiv3/go-lsp"
	"go.bug.st/json"
)

// NewReadWriteCloser create an io.ReadWriteCloser from given io.ReadCloser and io.WriteCloser.
func NewReadWriteCloser(in io.ReadCloser, out io.WriteCloser) io.ReadWriteCloser {
	return &combinedReadWriteCloser{in, out}
}

// OpenLogFileAs creates a log file in GlobalLogDirectory.
func openLogFileAs(filename string) *os.File {
	path := "./logs/" + filename
	res, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %s", err)
	}
	res.WriteString("\n\n\nStarted logging.\n")

	return res
}

func LogReadWriteCloserAs(upstream io.ReadWriteCloser, filename string) io.ReadWriteCloser {
	return &dumper{
		upstream: upstream,
		logfile:  openLogFileAs(filename),
	}
}

func catchAndLogPanic(callback func()) {
	if r := recover(); r != nil {
		reason := fmt.Sprintf("%v", r)
		LogError(fmt.Errorf("panic: %s\n\n%s", reason, string(debug.Stack())))

		go callback()
	}
}

func readConfig(configPath string) {
	if len(configPath) == 0 {
		configPath = "config.json"
	}
	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&config)
	if err != nil {
		panic(err)
	}
}

func applyTextmateMarks(uuid string, diagnostics *lsp.PublishDiagnosticsParams) {
	// Clear all marks first
	args := []string{"--uuid", uuid, "--clear-mark=note", "--clear-mark=warning", "--clear-mark=error"}
	cmd := exec.Command(config.MatePath, args...)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		LogError(err)
	}
	if config.EnableLogging {
		Log("Cleared marks: %s", stdoutStderr)
	}

	// Process each diagnostic
	for _, diag := range diagnostics.Diagnostics {
		// Check if range start is valid
		if diag.Range.Start.Line >= 0 {
			// Convert to 1-based indexing
			lineno := diag.Range.Start.Line + 1
			column := diag.Range.Start.Character + 1

			// Get appropriate icon based on severity
			icon := "error" // Default
			switch diag.Severity {
			case 4:
				icon = "note"
			case 3:
				icon = "warning"
			}
			lineArg := fmt.Sprintf("--line=%d:%d", lineno, column)
			markArg := fmt.Sprintf("--set-mark=%s:%s", icon, diag.Message)
			markArgs := []string{"--uuid", uuid, lineArg, markArg}
			cmd = exec.Command(config.MatePath, markArgs...)
			stdoutStderr, err = cmd.CombinedOutput()
			if err != nil {
				LogError(err)
			}
			if config.EnableLogging {
				Log("Marked: %s", stdoutStderr)
			}
		}
	}
}
