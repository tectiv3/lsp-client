package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"

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
		LogError(fmt.Errorf("Panic: %s\n\n%s", reason, string(debug.Stack())))

		go callback()
	}
}

func readConfig() {
	f, err := os.Open("config.json")
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
