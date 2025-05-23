package main

import (
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

var config Config

func main() {
	// check for first argument, if it is present - use it as config path
	if len(os.Args) > 1 {
		configPath := os.Args[1]
		readConfig(configPath)
	} else {
		readConfig("")
	}
	// start copilot LS
	copilotChan := make(mrChan, 2)
	go startCopilot(copilotChan)
	// start php intelephense LS
	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	// start go LS
	goplsChan := make(mrChan, 2)
	go startGopls(goplsChan)

	// start vue LS
	volarChan := make(mrChan, 2)
	go startVolar(volarChan)

	// start webserver
	go startServer(intelephenseChan, copilotChan, volarChan, goplsChan, config.Port)

	// wait for ctrl-c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	os.Exit(0)
}
