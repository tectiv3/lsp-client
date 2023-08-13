package main

import (
	"context"
	"go.bug.st/json"
	"go.bug.st/lsp"
	"go.bug.st/lsp/jsonrpc"
	"log"
)

type cmdHandler struct{}

func (h cmdHandler) ClientRegisterCapability(context.Context, jsonrpc.FunctionLogger, *lsp.RegistrationParams) *jsonrpc.ResponseError {
	return nil
}

// ClientUnregisterCapability
func (h cmdHandler) ClientUnregisterCapability(context.Context, jsonrpc.FunctionLogger, *lsp.UnregistrationParams) *jsonrpc.ResponseError {
	return nil
}

// LogTrace
func (h cmdHandler) LogTrace(logger jsonrpc.FunctionLogger, params *lsp.LogTraceParams) {
	log.Printf("LogTrace: %v", params)
}

// Progress
func (h cmdHandler) Progress(logger jsonrpc.FunctionLogger, params *lsp.ProgressParams) {
	log.Printf("Progress: %v", params)
}

// WindowShowMessage
func (h cmdHandler) WindowShowMessage(logger jsonrpc.FunctionLogger, params *lsp.ShowMessageParams) {
	log.Printf("WindowShowMessage: %v", params)
}

// WindowLogMessage
func (h cmdHandler) WindowLogMessage(logger jsonrpc.FunctionLogger, params *lsp.LogMessageParams) {
	logger.Logf("WindowLogMessage: %v", params)
}

// TelemetryEvent
func (h cmdHandler) TelemetryEvent(logger jsonrpc.FunctionLogger, msg json.RawMessage) {
	log.Printf("TelemetryEvent: %v", msg)
}

// TextDocumentPublishDiagnostics
func (h cmdHandler) TextDocumentPublishDiagnostics(logger jsonrpc.FunctionLogger, params *lsp.PublishDiagnosticsParams) {
	logger.Logf("TextDocumentPublishDiagnostics: %v", params)
}

// WindowShowMessageRequest
func (h cmdHandler) WindowShowMessageRequest(context.Context, jsonrpc.FunctionLogger, *lsp.ShowMessageRequestParams) (*lsp.MessageActionItem, *jsonrpc.ResponseError) {
	return nil, nil
}

// WindowShowDocument
func (h cmdHandler) WindowShowDocument(context.Context, jsonrpc.FunctionLogger, *lsp.ShowDocumentParams) (*lsp.ShowDocumentResult, *jsonrpc.ResponseError) {
	return nil, nil
}

// WindowWorkDoneProgressCreate
func (h cmdHandler) WindowWorkDoneProgressCreate(context.Context, jsonrpc.FunctionLogger, *lsp.WorkDoneProgressCreateParams) *jsonrpc.ResponseError {
	return nil
}

// WorkspaceWorkspaceFolders
func (h cmdHandler) WorkspaceWorkspaceFolders(context.Context, jsonrpc.FunctionLogger) ([]lsp.WorkspaceFolder, *jsonrpc.ResponseError) {
	return nil, nil
}

// WorkspaceConfiguration
func (h cmdHandler) WorkspaceConfiguration(context.Context, jsonrpc.FunctionLogger, *lsp.ConfigurationParams) ([]json.RawMessage, *jsonrpc.ResponseError) {
	return nil, nil
}

// WorkspaceApplyEdit
func (h cmdHandler) WorkspaceApplyEdit(context.Context, jsonrpc.FunctionLogger, *lsp.ApplyWorkspaceEditParams) (*lsp.ApplyWorkspaceEditResult, *jsonrpc.ResponseError) {
	return nil, nil
}

// WorkspaceCodeLensRefresh
func (h cmdHandler) WorkspaceCodeLensRefresh(context.Context, jsonrpc.FunctionLogger) *jsonrpc.ResponseError {
	return nil
}
