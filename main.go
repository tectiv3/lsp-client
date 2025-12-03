package main

import (
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

var config Config
var statusMonitor *StatusMonitor
var menuBarApp *MenuBarApp

func main() {
	// check for first argument, if it is present - use it as config path
	if len(os.Args) > 1 {
		configPath := os.Args[1]
		readConfig(configPath)
	} else {
		readConfig("")
	}

	// Create status channel for menu bar
	statusChan := make(chan AppStatus, 10)

	// start copilot LS
	copilotChan := make(mrChan, 2)
	copilotReadyChan := make(chan *handler, 1)
	go startCopilotWithStatus(copilotChan, copilotReadyChan)

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

	// Create ready channel to synchronize menu bar initialization
	menuBarReadyChan := make(chan struct{})

	// Wait for Copilot to be ready, then initialize status monitor and menu bar
	go func() {
		copilotHandler := <-copilotReadyChan
		auth := NewTerminalAuth(copilotHandler)

		statusMonitor = NewStatusMonitor(statusChan, copilotHandler, auth, &config)
		statusMonitor.Start()

		menuBarApp = NewMenuBarApp(statusChan, auth, &config)
		close(menuBarReadyChan)
	}()

	// Set up signal handler for Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		Log("Interrupt signal received, quitting...")
		if menuBarApp != nil {
			menuBarApp.Quit()
		} else {
			os.Exit(0)
		}
	}()

	// Wait for menu bar to be initialized
	<-menuBarReadyChan

	// Run menu bar on main thread - this blocks until quit
	menuBarApp.Run()

	os.Exit(0)
}
