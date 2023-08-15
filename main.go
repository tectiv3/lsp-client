package main

import (
	"encoding/json"
	"os"
	"os/signal"
)

var version = "0.0.1"

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

func main() {

	copilotChan := make(mrChan, 2)
	go startCopilot(copilotChan)
	sendLSPRequest(copilotChan, "initialize", KeyValue{})
	sendLSPRequest(copilotChan, "signIn", KeyValue{})
	sendLSPRequest(copilotChan, "getCompletions", KeyValue{})

	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	sendLSPRequest(intelephenseChan, "initialize", KeyValue{
		"dir":     "",
		"license": "",
		"name":    "",
	})

	// wait for ctrl-c

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	os.Exit(0)

	//startServer(intelephense, copilot, "8787")
}

func Log(format string, a ...interface{}) {
	logger.Logf(format, a...)
}

func LogError(err error) {
	logger.Logf(errorString("Error: %v", err))
}

func sendLSPRequest(out mrChan, method string, params KeyValue) {
	cb := make(kvChan)
	body, _ := json.Marshal(params)
	out <- &mateRequest{
		Method: method,
		Body:   body,
		CB:     cb,
	}

	if result := <-cb; result != nil {
		Log("Result: %v", result)
	}
}
