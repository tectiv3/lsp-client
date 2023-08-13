package main

var version = "0.0.1"

func main() {

	go startIntelephense("../backend", "")
	startCopilot("../backend")

}
