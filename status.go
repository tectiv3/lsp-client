package main

import (
	"context"
	"os"
	"sync"
	"time"

	"go.bug.st/json"
)

type StatusMonitor struct {
	statusChan     chan AppStatus
	copilotHandler *handler
	authHandler    *TerminalAuth
	config         *Config
	mu             sync.RWMutex
	currentStatus  AppStatus
	stopChan       chan struct{}
}

func NewStatusMonitor(statusChan chan AppStatus, copilot *handler, auth *TerminalAuth, cfg *Config) *StatusMonitor {
	wd, _ := os.Getwd()

	// Determine initial status based on configuration
	copilotStatus := ServerStatusRunning
	if len(cfg.CopilotPath) == 0 {
		copilotStatus = ServerStatusStopped
	}

	intelephenseStatus := ServerStatusRunning
	if len(cfg.IntelephensePath) == 0 {
		intelephenseStatus = ServerStatusStopped
	}

	goplsStatus := ServerStatusRunning
	if len(cfg.GoplsPath) == 0 {
		goplsStatus = ServerStatusStopped
	}

	volarStatus := ServerStatusRunning
	if len(cfg.VolarPath) == 0 {
		volarStatus = ServerStatusStopped
	}

	return &StatusMonitor{
		statusChan:     statusChan,
		copilotHandler: copilot,
		authHandler:    auth,
		config:         cfg,
		stopChan:       make(chan struct{}),
		currentStatus: AppStatus{
			CopilotStatus:      copilotStatus,
			IntelephenseStatus: intelephenseStatus,
			GoplsStatus:        goplsStatus,
			VolarStatus:        volarStatus,
			HTTPServerStatus:   ServerStatusRunning,
			HTTPPort:           cfg.Port,
			WorkingDirectory:   wd,
			LastUpdate:         time.Now(),
		},
	}
}

func (sm *StatusMonitor) Start() {
	go sm.monitorLoop()
}

func (sm *StatusMonitor) Stop() {
	close(sm.stopChan)
}

func (sm *StatusMonitor) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	sm.updateStatus()

	for {
		select {
		case <-ticker.C:
			sm.updateStatus()
		case <-sm.stopChan:
			return
		}
	}
}

func (sm *StatusMonitor) updateStatus() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	authStatus := sm.checkAuthStatus()
	sm.currentStatus.Auth = authStatus
	sm.currentStatus.LastUpdate = time.Now()

	sm.broadcastStatus()
}

func (sm *StatusMonitor) checkAuthStatus() AuthStatus {
	if sm.copilotHandler == nil || sm.copilotHandler.lsc == nil {
		return AuthStatus{
			IsAuthenticated: false,
			Status:          "Not Connected",
		}
	}

	ctx := context.Background()
	conn := sm.copilotHandler.lsc.GetConnection()

	resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)
	var checkResult checkStatusResponse
	if err := json.Unmarshal(resp, &checkResult); err != nil {
		Log("Failed to check auth status: %v", err)
		return AuthStatus{
			IsAuthenticated: false,
			Status:          "Error",
		}
	}

	if checkResult.Status == "OK" || checkResult.Status == "AlreadySignedIn" {
		return AuthStatus{
			IsAuthenticated: true,
			Username:        checkResult.User,
			Status:          checkResult.Status,
			NeedsAuth:       false,
		}
	}

	if checkResult.Status == "NotAuthorized" {
		signInResp := sm.checkForPendingAuth()
		if signInResp != nil && signInResp.UserCode != "" {
			return AuthStatus{
				IsAuthenticated: false,
				Status:          "NotAuthorized",
				UserCode:        signInResp.UserCode,
				VerificationUri: signInResp.VerificationUri,
				NeedsAuth:       true,
			}
		}
		return AuthStatus{
			IsAuthenticated: false,
			Status:          "NotAuthorized",
			NeedsAuth:       true,
		}
	}

	return AuthStatus{
		IsAuthenticated: false,
		Status:          checkResult.Status,
	}
}

func (sm *StatusMonitor) checkForPendingAuth() *signInResponse {
	if sm.copilotHandler == nil || sm.copilotHandler.lsc == nil {
		return nil
	}

	ctx := context.Background()
	conn := sm.copilotHandler.lsc.GetConnection()

	resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
	var signInResp signInResponse
	if err := json.Unmarshal(resp, &signInResp); err != nil {
		return nil
	}

	if signInResp.Status == "PromptUserDeviceFlow" && signInResp.UserCode != "" {
		return &signInResp
	}

	return nil
}

func (sm *StatusMonitor) broadcastStatus() {
	select {
	case sm.statusChan <- sm.currentStatus:
	default:
	}
}

func (sm *StatusMonitor) UpdateServerStatus(serverName string, status ServerStatus) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch serverName {
	case "copilot":
		sm.currentStatus.CopilotStatus = status
	case "intelephense":
		sm.currentStatus.IntelephenseStatus = status
	case "gopls":
		sm.currentStatus.GoplsStatus = status
	case "volar":
		sm.currentStatus.VolarStatus = status
	case "http":
		sm.currentStatus.HTTPServerStatus = status
	}

	sm.currentStatus.LastUpdate = time.Now()
	sm.broadcastStatus()
}

func (sm *StatusMonitor) GetCurrentStatus() AppStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentStatus
}
