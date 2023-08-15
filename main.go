package main

import "encoding/json"

var version = "0.0.1"

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

func main() {

	startIntelephense("", "")
	requestChan := make(mrChan, 2)
	go startCopilot(requestChan)
	sendCopilotRequest(requestChan, "initialize", KeyValue{})
	sendCopilotRequest(requestChan, "signIn", KeyValue{})
	sendCopilotRequest(requestChan, "getCompletions", KeyValue{})

	//startServer(intelephense, copilot, "8787")
}

func Log(format string, a ...interface{}) {
	logger.Logf(format, a...)
}

func sendCopilotRequest(out mrChan, method string, params KeyValue) {
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
