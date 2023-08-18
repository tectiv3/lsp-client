package main

import (
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

var config Config

func main() {
	readConfig()
	// start copilot LS
	copilotChan := make(mrChan, 2)
	go startCopilot(copilotChan)
	// start intelephense LS
	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	// start webserver
	go startServer(intelephenseChan, copilotChan, config.Port)

	// wait for ctrl-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	os.Exit(0)
}
