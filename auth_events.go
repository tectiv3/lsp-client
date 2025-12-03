package main

import "sync"

var (
	authStatusUpdateChannel chan struct{}
	authStatusMu            sync.RWMutex
)

func setAuthStatusUpdateChannel(ch chan struct{}) {
	authStatusMu.Lock()
	defer authStatusMu.Unlock()
	authStatusUpdateChannel = ch
}

func notifyAuthStatusChanged() {
	authStatusMu.RLock()
	ch := authStatusUpdateChannel
	authStatusMu.RUnlock()

	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
		// Update already pending
	}
}
