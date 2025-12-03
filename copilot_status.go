package main

import (
	"context"
	"time"

	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

const copilotStatusTimeout = 10 * time.Second

func FetchCopilotAuthStatus(h *handler) AuthStatus {
	if h == nil || h.lsc == nil {
		return AuthStatus{IsAuthenticated: false, Status: "Not Connected"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), copilotStatusTimeout)
	defer cancel()

	conn := h.lsc.GetConnection()
	resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)
	var checkResult checkStatusResponse
	if err := json.Unmarshal(resp, &checkResult); err != nil {
		Log("Failed to check auth status: %v", err)
		return AuthStatus{IsAuthenticated: false, Status: "Error"}
	}

	switch checkResult.Status {
	case "OK", "AlreadySignedIn":
		return AuthStatus{
			IsAuthenticated: true,
			Username:        checkResult.User,
			Status:          checkResult.Status,
			NeedsAuth:       false,
		}
	case "NotAuthorized":
		if pending := fetchPendingAuthCode(conn); pending != nil {
			return AuthStatus{
				IsAuthenticated: false,
				Status:          checkResult.Status,
				UserCode:        pending.UserCode,
				VerificationUri: pending.VerificationUri,
				NeedsAuth:       true,
			}
		}
		return AuthStatus{
			IsAuthenticated: false,
			Status:          checkResult.Status,
			NeedsAuth:       true,
		}
	case "NotSignedIn", "NotAuthenticated":
		return AuthStatus{
			IsAuthenticated: false,
			Status:          checkResult.Status,
			NeedsAuth:       true,
		}
	default:
		return AuthStatus{
			IsAuthenticated: false,
			Status:          checkResult.Status,
		}
	}
}

func fetchPendingAuthCode(conn *jsonrpc.Connection) *signInResponse {
	ctx, cancel := context.WithTimeout(context.Background(), copilotStatusTimeout)
	defer cancel()

	resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
	var signInResp signInResponse
	if err := json.Unmarshal(resp, &signInResp); err != nil {
		Log("Failed to check pending auth: %v", err)
		return nil
	}

	if signInResp.Status == "PromptUserDeviceFlow" && signInResp.UserCode != "" {
		return &signInResp
	}
	return nil
}
