package main

import (
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

func main() {
	copilotChan := make(mrChan, 2)
	go startCopilot(copilotChan)
	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	go startServer(intelephenseChan, copilotChan, "8787")

	// wait for ctrl-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	os.Exit(0)
}

func Log(format string, a ...interface{}) {
	logger.Logf(format, a...)
}

func LogError(err error) {
	logger.Logf(errorString("Error: %v", err))
}
