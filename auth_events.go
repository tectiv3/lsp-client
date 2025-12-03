package main

var authStatusUpdateChannel chan struct{}

func setAuthStatusUpdateChannel(ch chan struct{}) {
	authStatusUpdateChannel = ch
}

func notifyAuthStatusChanged() {
	ch := authStatusUpdateChannel
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
		<-ch
		ch <- struct{}{}
	}
}
