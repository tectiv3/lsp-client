package main

import (
	"go.bug.st/json"
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

var config Config

func main() {
	readConfig()

	copilotChan := make(mrChan, 2)
	go startCopilot(copilotChan)
	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	go startServer(intelephenseChan, copilotChan, config.Port)

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
