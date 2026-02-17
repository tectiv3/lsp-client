package main

import (
	"fmt"
	"os"
	"os/signal"
)

var logger = NewLSPFunctionLogger(hiMagentaString, "App")

var config Config
var menuBarApp *MenuBarApp

func main() {
	var configPath string
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	if needsSetup(configPath) {
		cfg, err := runSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
		config = *cfg
	} else {
		readConfig(configPath)
	}

	startCopilotTokenMaintenance()

	authStatusUpdates := make(chan struct{}, 1)
	setAuthStatusUpdateChannel(authStatusUpdates)

	startedServers := []string{}

	// start copilot LS
	copilotChan := make(mrChan, 2)
	copilotReadyChan := make(chan *handler, 1)
	go startCopilotWithStatus(copilotChan, copilotReadyChan)
	if len(config.CopilotPath) > 0 {
		startedServers = append(startedServers, "Copilot")
	}

	// start php intelephense LS
	intelephenseChan := make(mrChan, 2)
	go startIntelephense(intelephenseChan)
	if len(config.IntelephensePath) > 0 {
		startedServers = append(startedServers, "Intelephense")
	}

	// start go LS
	goplsChan := make(mrChan, 2)
	go startGopls(goplsChan)
	if len(config.GoplsPath) > 0 {
		startedServers = append(startedServers, "Gopls")
	}

	// start vue LS
	vueChan := make(mrChan, 2)
	go startVue(vueChan)
	if len(config.VueLSPath) > 0 {
		startedServers = append(startedServers, "Vue")
	}

	// start webserver
	go startServer(intelephenseChan, copilotChan, vueChan, goplsChan, config.Port)
	if config.Port != "" {
		startedServers = append(startedServers, fmt.Sprintf("HTTP Server (:%s)", config.Port))
	} else {
		startedServers = append(startedServers, "HTTP Server")
	}

	// Create ready channel to synchronize menu bar initialization
	menuBarReadyChan := make(chan struct{})

	// Wait for Copilot to be ready, then initialize menu bar
	go func() {
		copilotHandler := <-copilotReadyChan
		auth := NewTerminalAuth(copilotHandler)

		menuBarApp = NewMenuBarApp(startedServers, authStatusUpdates, auth, &config)
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
