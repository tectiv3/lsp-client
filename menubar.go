package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
)

type MenuBarApp struct {
	authHandler    *TerminalAuth
	config         *Config
	startedServers []string
	authUpdates    <-chan struct{}

	stopChan chan struct{}
	stopOnce sync.Once

	mu         sync.RWMutex
	authStatus AuthStatus

	copilotItem  *systray.MenuItem
	authCodeItem *systray.MenuItem
	serversMenu  *systray.MenuItem
	logsItem     *systray.MenuItem
	reauthItem   *systray.MenuItem
	quitItem     *systray.MenuItem
}

func NewMenuBarApp(startedServers []string, authUpdates <-chan struct{}, authHandler *TerminalAuth, cfg *Config) *MenuBarApp {
	return &MenuBarApp{
		authHandler:    authHandler,
		config:         cfg,
		startedServers: startedServers,
		authUpdates:    authUpdates,
		stopChan:       make(chan struct{}),
	}
}

func (m *MenuBarApp) Run() {
	systray.Run(m.onReady, m.onExit)
}

func (m *MenuBarApp) onReady() {
	systray.SetTitle("{}")
	systray.SetTooltip("LSP Client")

	systray.AddSeparator()
	m.copilotItem = systray.AddMenuItem("Copilot: Checking...", "GitHub Copilot authentication status")
	m.authCodeItem = systray.AddMenuItem("", "")
	m.authCodeItem.Hide()

	systray.AddSeparator()
	m.reauthItem = systray.AddMenuItem("Re-authenticate Copilot", "Trigger Copilot re-authentication")

	systray.AddSeparator()
	m.serversMenu = systray.AddMenuItem("Started Servers", "Language servers launched for this session")
	m.populateServersMenu()

	systray.AddSeparator()
	m.logsItem = systray.AddMenuItem("Open Logs", "Open logs directory")

	systray.AddSeparator()
	m.quitItem = systray.AddMenuItem("Quit", "Quit the application")

	go m.handleMenuActions()
	if m.authUpdates != nil {
		go m.listenAuthUpdates()
	}
	m.refreshAuthStatusAsync()
}

func (m *MenuBarApp) onExit() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

func (m *MenuBarApp) listenAuthUpdates() {
	for {
		select {
		case <-m.authUpdates:
			m.refreshAuthStatusAsync()
		case <-m.stopChan:
			return
		}
	}
}

func (m *MenuBarApp) populateServersMenu() {
	if len(m.startedServers) == 0 {
		item := m.serversMenu.AddSubMenuItem("• No servers started", "No language servers were launched")
		item.Disable()
		return
	}

	for _, name := range m.startedServers {
		item := m.serversMenu.AddSubMenuItem(fmt.Sprintf("• %s", name), fmt.Sprintf("%s server", name))
		item.Disable()
	}
}

func (m *MenuBarApp) handleMenuActions() {
	for {
		select {
		case <-m.quitItem.ClickedCh:
			Log("Menu bar: Quit requested")
			systray.Quit()

		case <-m.authCodeItem.ClickedCh:
			status := m.getAuthStatus()
			if status.UserCode != "" {
				if err := clipboard.WriteAll(status.UserCode); err != nil {
					Log("Failed to copy auth code: %v", err)
				} else {
					Log("Auth code copied to clipboard")
					m.authCodeItem.SetTitle(fmt.Sprintf("Code copied: %s", status.UserCode))
					go func(userCode string) {
						time.Sleep(2 * time.Second)
						current := m.getAuthStatus()
						if current.UserCode == userCode {
							m.authCodeItem.SetTitle(fmt.Sprintf("Click to copy: %s", userCode))
						}
					}(status.UserCode)
				}
			}

		case <-m.reauthItem.ClickedCh:
			Log("Menu bar: Manual re-authentication requested")
			go m.performReauth()

		case <-m.logsItem.ClickedCh:
			m.openLogsDirectory()

		case <-m.stopChan:
			return
		}
	}
}

func (m *MenuBarApp) performReauth() {
	if m.authHandler == nil {
		return
	}

	m.reauthItem.SetTitle("Authenticating...")
	m.reauthItem.Disable()

	resp, err := m.authHandler.InitiateAuth()
	if err != nil {
		Log("Re-authentication failed: %v", err)
		m.reauthItem.SetTitle("Re-authenticate Copilot (Failed)")
	} else {
		if resp.Status == "AlreadySignedIn" {
			Log("Already signed in")
		} else {
			if err := m.authHandler.openBrowser(resp.VerificationUri); err != nil {
				Log("Failed to open browser: %v", err)
			}
			// Update UI to show code immediately
			m.authCodeItem.SetTitle(fmt.Sprintf("📋 Click to copy: %s", resp.UserCode))
			m.authCodeItem.Show()
		}
		m.reauthItem.SetTitle("Re-authenticate Copilot")
	}

	m.reauthItem.Enable()
	m.refreshAuthStatusAsync()
}

func (m *MenuBarApp) refreshAuthStatusAsync() {
	go func() {
		var copilotHandler *handler
		if m.authHandler != nil {
			copilotHandler = m.authHandler.client
		}
		status := FetchCopilotAuthStatus(copilotHandler)
		m.setAuthStatus(status)
		m.updateAuthMenu(status)
	}()
}

func (m *MenuBarApp) updateAuthMenu(status AuthStatus) {
	if status.IsAuthenticated {
		m.copilotItem.SetTitle(fmt.Sprintf("✓ Copilot: %s", status.Username))
		m.authCodeItem.Hide()
	} else if status.NeedsAuth && status.UserCode != "" {
		m.copilotItem.SetTitle("⚠️  Authentication Required")
		m.authCodeItem.SetTitle(fmt.Sprintf("📋 Click to copy: %s", status.UserCode))
		m.authCodeItem.Show()
	} else if status.Status == "NotAuthorized" {
		m.copilotItem.SetTitle("❌ Copilot: Not Authenticated")
		m.authCodeItem.Hide()
	} else {
		m.copilotItem.SetTitle("⏳ Copilot: Checking...")
		m.authCodeItem.Hide()
	}
}

func (m *MenuBarApp) setAuthStatus(status AuthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authStatus = status
}

func (m *MenuBarApp) getAuthStatus() AuthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authStatus
}

func (m *MenuBarApp) openLogsDirectory() {
	logsPath := "./logs"
	absPath, err := filepath.Abs(logsPath)
	if err != nil {
		Log("Failed to get absolute path for logs: %v", err)
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		Log("Logs directory does not exist: %s", absPath)
		return
	}

	cmd := exec.Command("open", absPath)
	if err := cmd.Start(); err != nil {
		Log("Failed to open logs directory: %v", err)
	} else {
		Log("Opened logs directory: %s", absPath)
	}
}

func (m *MenuBarApp) Quit() {
	systray.Quit()
}
