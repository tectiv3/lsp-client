package main

import (
	"context"
	"fmt"
	lsp "github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"
)

type cmdHandler struct {
	lsc                   *lsp.Client
	Diagnostics           chan *lsp.PublishDiagnosticsParams
	waitingForDiagnostics bool
	sync.Mutex
}

func (h cmdHandler) GetDiagnosticChannel() chan *lsp.PublishDiagnosticsParams {
	return h.Diagnostics
}

func (h cmdHandler) ClientRegisterCapability(context.Context, jsonrpc.FunctionLogger, *lsp.RegistrationParams) *jsonrpc.ResponseError {
	//Log("ClientRegisterCapability")
	//h.client.GetConnection().SendNotification("client/registerCapability", lsp.EncodeMessage(KeyValue{}))
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
func (h cmdHandler) WorkspaceConfiguration(context.Context, jsonrpc.FunctionLogger, *lsp.ConfigurationParams) ([]json.RawMessage, *jsonrpc.ResponseError) {
	cfg := KeyValue{
		"files": KeyValue{
			"maxSize":      300000,
			"associations": []string{"*.php", "*.phtml"},
			"exclude": []string{
				"**/.git/**",
				"**/.svn/**",
				"**/.hg/**",
				"**/CVS/**",
				"**/.DS_Store/**",
				"**/node_modules/**",
				"**/bower_components/**",
				"**/vendor/**/{Test,test,Tests,tests}/**",
				"**/.git",
				"**/.svn",
				"**/.hg",
				"**/CVS",
				"**/.DS_Store",
				"**/nova/tests/**",
				"**/faker/**",
				"**/*.log",
				"**/*.log*",
				"**/*.min.*",
				"**/dist",
				"**/coverage",
				"**/build/*",
				"**/nova/public/*",
				"**/public/*",
			},
		},
		"stubs": []string{
			"apache",
			"bcmath",
			"bz2",
			"calendar",
			"com_dotnet",
			"Core",
			"ctype",
			"curl",
			"date",
			"dba",
			"dom",
			"enchant",
			"exif",
			"fileinfo",
			"filter",
			"fpm",
			"ftp",
			"gd",
			"hash",
			"iconv",
			"imap",
			"interbase",
			"intl",
			"json",
			"ldap",
			"libxml",
			"mbstring",
			"mcrypt",
			"meta",
			"mssql",
			"mysqli",
			"oci8",
			"odbc",
			"openssl",
			"pcntl",
			"pcre",
			"PDO",
			"pdo_ibm",
			"pdo_mysql",
			"pdo_pgsql",
			"pdo_sqlite",
			"pgsql",
			"Phar",
			"posix",
			"pspell",
			"readline",
			"recode",
			"Reflection",
			"regex",
			"session",
			"shmop",
			"SimpleXML",
			"snmp",
			"soap",
			"sockets",
			"sodium",
			"SPL",
			"sqlite3",
			"standard",
			"superglobals",
			"sybase",
			"sysvmsg",
			"sysvsem",
			"sysvshm",
			"tidy",
			"tokenizer",
			"wddx",
			"xml",
			"xmlreader",
			"xmlrpc",
			"xmlwriter",
			"Zend OPcache",
			"zip",
			"zlib",
		},
		"completion": KeyValue{
			"insertUseDeclaration":                    true,
			"fullyQualifyGlobalConstantsAndFunctions": false,
			"triggerParameterHints":                   true,
			"maxItems":                                100,
		},
		"format": KeyValue{
			"enable": false,
		},
		"environment": KeyValue{
			"documentRoot": "",
			"includePaths": []string{},
		},
		"runtime":   "",
		"maxMemory": 0,
		"telemetry": KeyValue{"enabled": false},
		"trace": KeyValue{
			"server": "verbose",
		},
	}

	body, _ := json.Marshal(cfg)

	return []json.RawMessage{body, body}, nil
}

// WorkspaceApplyEdit
func (h cmdHandler) WorkspaceApplyEdit(context.Context, jsonrpc.FunctionLogger, *lsp.ApplyWorkspaceEditParams) (*lsp.ApplyWorkspaceEditResult, *jsonrpc.ResponseError) {
	return nil, nil
}

// WorkspaceCodeLensRefresh
func (h cmdHandler) WorkspaceCodeLensRefresh(context.Context, jsonrpc.FunctionLogger) *jsonrpc.ResponseError {
	return nil
}

// Panicf takes the return value of recover() and outputs data to the log with
// the stack trace appended. Arguments are handled in the manner of
// fmt.Printf. Arguments should format to a string which identifies what the
// panic code was doing. Returns a non-nil error if it recovered from a panic.
func Panicf(r interface{}, format string, v ...interface{}) error {
	if r != nil {
		// Same as net/http
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		id := fmt.Sprintf(format, v...)
		log.Printf("panic serving %s: %v\n%s", id, r, string(buf))
		return fmt.Errorf("unexpected panic: %v", r)
	}
	return nil
}

func startRPCServer(app, name string, args ...string) *cmdHandler {
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
	stdio = LogReadWriteCloserAs(stdio, app+".log")
	go io.Copy(openLogFileAs(app+"-err.log"), stderr)

	handler := &cmdHandler{
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
