package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
)

type MenuBarApp struct {
	statusChan    chan AppStatus
	authHandler   *TerminalAuth
	config        *Config
	stopChan      chan struct{}
	currentStatus AppStatus
	copilotItem   *systray.MenuItem
	authCodeItem  *systray.MenuItem
	serversMenu   *systray.MenuItem
	copilotServer *systray.MenuItem
	intelephense  *systray.MenuItem
	gopls         *systray.MenuItem
	volar         *systray.MenuItem
	httpServer    *systray.MenuItem
	logsItem      *systray.MenuItem
	reauthItem    *systray.MenuItem
	quitItem      *systray.MenuItem
}

func NewMenuBarApp(statusChan chan AppStatus, authHandler *TerminalAuth, cfg *Config) *MenuBarApp {
	return &MenuBarApp{
		statusChan:  statusChan,
		authHandler: authHandler,
		config:      cfg,
		stopChan:    make(chan struct{}),
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
	m.serversMenu = systray.AddMenuItem("Server Status", "Language server status")

	// Only create menu items for configured servers
	if len(m.config.CopilotPath) > 0 {
		m.copilotServer = m.serversMenu.AddSubMenuItem("● Copilot: Starting...", "Copilot language server")
	}
	if len(m.config.IntelephensePath) > 0 {
		m.intelephense = m.serversMenu.AddSubMenuItem("● Intelephense: Starting...", "PHP language server")
	}
	if len(m.config.GoplsPath) > 0 {
		m.gopls = m.serversMenu.AddSubMenuItem("● Gopls: Starting...", "Go language server")
	}
	if len(m.config.VolarPath) > 0 {
		m.volar = m.serversMenu.AddSubMenuItem("● Volar: Starting...", "Vue language server")
	}
	m.httpServer = m.serversMenu.AddSubMenuItem("● HTTP Server: Starting...", "TextMate HTTP server")

	systray.AddSeparator()
	m.logsItem = systray.AddMenuItem("Open Logs", "Open logs directory")

	systray.AddSeparator()
	m.quitItem = systray.AddMenuItem("Quit", "Quit the application")

	go m.handleMenuActions()
	go m.updateStatus()
}

func (m *MenuBarApp) onExit() {
	close(m.stopChan)
	if statusMonitor != nil {
		statusMonitor.Stop()
	}
}

func (m *MenuBarApp) handleMenuActions() {
	for {
		select {
		case <-m.quitItem.ClickedCh:
			Log("Menu bar: Quit requested")
			systray.Quit()

		case <-m.authCodeItem.ClickedCh:
			if m.currentStatus.Auth.UserCode != "" {
				if err := clipboard.WriteAll(m.currentStatus.Auth.UserCode); err != nil {
					Log("Failed to copy auth code: %v", err)
				} else {
					Log("Auth code copied to clipboard")
					m.authCodeItem.SetTitle(fmt.Sprintf("Code copied: %s", m.currentStatus.Auth.UserCode))
					go func() {
						time.Sleep(2 * time.Second)
						if m.currentStatus.Auth.UserCode != "" {
							m.authCodeItem.SetTitle(fmt.Sprintf("Click to copy: %s", m.currentStatus.Auth.UserCode))
						}
					}()
				}
			}

		case <-m.reauthItem.ClickedCh:
			Log("Menu bar: Manual re-authentication requested")
			go func() {
				if m.authHandler != nil {
					m.reauthItem.SetTitle("Authenticating...")
					m.reauthItem.Disable()
					if err := m.authHandler.PerformTerminalAuth(); err != nil {
						Log("Re-authentication failed: %v", err)
						m.reauthItem.SetTitle("Re-authenticate Copilot (Failed)")
					} else {
						m.reauthItem.SetTitle("Re-authenticate Copilot")
					}
					m.reauthItem.Enable()
				}
			}()

		case <-m.logsItem.ClickedCh:
			m.openLogsDirectory()

		case <-m.stopChan:
			return
		}
	}
}

func (m *MenuBarApp) updateStatus() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case status := <-m.statusChan:
			m.currentStatus = status
			m.refreshMenu()
		case <-ticker.C:
		case <-m.stopChan:
			return
		}
	}
}

func (m *MenuBarApp) refreshMenu() {
	status := m.currentStatus

	if status.Auth.IsAuthenticated {
		m.copilotItem.SetTitle(fmt.Sprintf("✓ Copilot: %s", status.Auth.Username))
		m.authCodeItem.Hide()
	} else if status.Auth.NeedsAuth && status.Auth.UserCode != "" {
		m.copilotItem.SetTitle("⚠️  Authentication Required")
		m.authCodeItem.SetTitle(fmt.Sprintf("📋 Click to copy: %s", status.Auth.UserCode))
		m.authCodeItem.Show()
	} else if status.Auth.Status == "NotAuthorized" {
		m.copilotItem.SetTitle("❌ Copilot: Not Authenticated")
		m.authCodeItem.Hide()
	} else {
		m.copilotItem.SetTitle("⏳ Copilot: Checking...")
		m.authCodeItem.Hide()
	}

	if m.copilotServer != nil {
		m.updateServerMenuItem(m.copilotServer, "Copilot", status.CopilotStatus)
	}
	if m.intelephense != nil {
		m.updateServerMenuItem(m.intelephense, "Intelephense", status.IntelephenseStatus)
	}
	if m.gopls != nil {
		m.updateServerMenuItem(m.gopls, "Gopls", status.GoplsStatus)
	}
	if m.volar != nil {
		m.updateServerMenuItem(m.volar, "Volar", status.VolarStatus)
	}

	if status.HTTPPort != "" {
		m.updateServerMenuItem(m.httpServer, fmt.Sprintf("HTTP Server :%s", status.HTTPPort), status.HTTPServerStatus)
	} else {
		m.updateServerMenuItem(m.httpServer, "HTTP Server", status.HTTPServerStatus)
	}
}

func (m *MenuBarApp) updateServerMenuItem(item *systray.MenuItem, name string, status ServerStatus) {
	var icon string
	switch status {
	case ServerStatusRunning:
		icon = "🟢"
	case ServerStatusIdle:
		icon = "🟡"
	case ServerStatusError:
		icon = "🔴"
	case ServerStatusStopped:
		icon = "⚫"
	default:
		icon = "⚪"
	}
	item.SetTitle(fmt.Sprintf("%s %s: %s", icon, name, status))
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
