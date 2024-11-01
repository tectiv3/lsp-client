package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	lsp "github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

type handler struct {
	lsc                   *lsp.Client
	Diagnostics           chan *lsp.PublishDiagnosticsParams
	waitingForDiagnostics bool
	config                KeyValue
	sync.Mutex
}

func (h handler) SetConfig(config KeyValue) {
	h.config = config
}

func (h handler) GetDiagnosticChannel() chan *lsp.PublishDiagnosticsParams {
	return h.Diagnostics
}

func (h handler) ClientRegisterCapability(context.Context, jsonrpc.FunctionLogger, *lsp.RegistrationParams) *jsonrpc.ResponseError {
	//Log("ClientRegisterCapability")
	//h.client.GetConnection().SendNotification("client/registerCapability", lsp.EncodeMessage(KeyValue{}))
	return nil
}

// ClientUnregisterCapability
func (h handler) ClientUnregisterCapability(context.Context, jsonrpc.FunctionLogger, *lsp.UnregistrationParams) *jsonrpc.ResponseError {
	return nil
}

// LogTrace
func (h handler) LogTrace(logger jsonrpc.FunctionLogger, params *lsp.LogTraceParams) {
	log.Printf("LogTrace: %v", params)
}

// Progress
func (h handler) Progress(logger jsonrpc.FunctionLogger, params *lsp.ProgressParams) {
	log.Printf("Progress: %v", params)
}

// WindowShowMessage
func (h handler) WindowShowMessage(logger jsonrpc.FunctionLogger, params *lsp.ShowMessageParams) {
	log.Printf("WindowShowMessage: %v", params)
}

// WindowLogMessage
func (h handler) WindowLogMessage(logger jsonrpc.FunctionLogger, params *lsp.LogMessageParams) {
	logger.Logf("%v", params)
}

// TelemetryEvent
func (h handler) TelemetryEvent(logger jsonrpc.FunctionLogger, msg json.RawMessage) {
	log.Printf("TelemetryEvent: %v", msg)
}

// TextDocumentPublishDiagnostics
func (h handler) TextDocumentPublishDiagnostics(logger jsonrpc.FunctionLogger, params *lsp.PublishDiagnosticsParams) {
	go func() {
		h.Lock()
		defer h.Unlock()
		if !h.waitingForDiagnostics {
			return
		}
		logger.Logf("TextDocumentPublishDiagnostics: %v", params)
		if params.IsClear {
			logger.Logf("Clearing diagnostics for %s", params.URI)
			return
		}
		h.Diagnostics <- params
	}()
}

// WindowShowMessageRequest
func (h handler) WindowShowMessageRequest(context.Context, jsonrpc.FunctionLogger, *lsp.ShowMessageRequestParams) (*lsp.MessageActionItem, *jsonrpc.ResponseError) {
	return nil, nil
}

// WindowShowDocument
func (h handler) WindowShowDocument(context.Context, jsonrpc.FunctionLogger, *lsp.ShowDocumentParams) (*lsp.ShowDocumentResult, *jsonrpc.ResponseError) {
	return nil, nil
}

// WindowWorkDoneProgressCreate
func (h handler) WindowWorkDoneProgressCreate(context.Context, jsonrpc.FunctionLogger, *lsp.WorkDoneProgressCreateParams) *jsonrpc.ResponseError {
	return nil
}

// WorkspaceWorkspaceFolders
func (h handler) WorkspaceWorkspaceFolders(context.Context, jsonrpc.FunctionLogger) ([]lsp.WorkspaceFolder, *jsonrpc.ResponseError) {
	folders := []lsp.WorkspaceFolder{}
	// go over server openFolders and append to folders
	for name, folder := range server.openFolders {
		folders = append(folders, lsp.WorkspaceFolder{
			URI:  folder,
			Name: name,
		})
	}

	return folders, nil
}

// WorkspaceConfiguration
func (h handler) WorkspaceConfiguration(context.Context, jsonrpc.FunctionLogger, *lsp.ConfigurationParams) ([]json.RawMessage, *jsonrpc.ResponseError) {
	body, _ := json.Marshal(h.config)

	return []json.RawMessage{body, body}, nil
}

// WorkspaceApplyEdit
func (h handler) WorkspaceApplyEdit(context.Context, jsonrpc.FunctionLogger, *lsp.ApplyWorkspaceEditParams) (*lsp.ApplyWorkspaceEditResult, *jsonrpc.ResponseError) {
	return nil, nil
}

// WorkspaceCodeLensRefresh
func (h handler) WorkspaceCodeLensRefresh(context.Context, jsonrpc.FunctionLogger) *jsonrpc.ResponseError {
	return nil
}

func startRPCServer(app, name string, args ...string) *handler {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser

	cmd := exec.Command(name, args...)

	if cin, err := cmd.StdinPipe(); err != nil {
		panic("getting clangd stdin: " + err.Error())
	} else if cout, err := cmd.StdoutPipe(); err != nil {
		panic("getting clangd stdout: " + err.Error())
	} else if cerr, err := cmd.StderrPipe(); err != nil {
		panic("getting clangd stderr: " + err.Error())
	} else if err := cmd.Start(); err != nil {
		panic("running clangd: " + err.Error())
	} else {
		stdin = cin
		stdout = cout
		stderr = cerr
	}

	stdio := NewReadWriteCloser(stdout, stdin)
	if config.EnableLogging {
		stdio = LogReadWriteCloserAs(stdio, app+".log")
		go io.Copy(openLogFileAs(app+"-err.log"), stderr)
	} else {
		go io.Copy(os.Stderr, stderr)
	}

	handler := &handler{
		Diagnostics: make(chan *lsp.PublishDiagnosticsParams),
	}
	lsc := lsp.NewClient(stdio, stdio, handler, func(err error) {
		log.Println(errorString("Error: %v", err))
	})
	handler.lsc = lsc

	go func() {
		defer stdin.Close()
		cmd.Wait()
	}()

	return handler
}
