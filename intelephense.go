package main

import (
	"context"
	lsp "github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"log"
	"os"
	"os/exec"
	"time"
)

func startIntelephense(dir string, license string) {
	cmd := exec.Command("/opt/homebrew/opt/node@16/bin/node", "/opt/homebrew/bin/intelephense", "--stdio")

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	lsc := lsp.NewClient(stdout, stdin, cmdHandler{})
	ctx := context.Background()
	go lsc.Run()

	//conn := lsc.GetConnection()
	pid := os.Getpid()

	log.Println(lsp.NewDocumentURI(dir))
	response, respErr, err := lsc.Initialize(ctx, &lsp.InitializeParams{
		ProcessID:             &pid,
		RootURI:               lsp.NewDocumentURI(dir),
		RootPath:              dir,
		InitializationOptions: lsp.KeyValue{"storagePath": "/tmp/intelephense/", "clearCache": true, "isVscode": true, "licenceKey": license},
		Capabilities: lsp.ClientCapabilities{
			WorkspaceFolders: []lsp.KeyValue{
				{
					"uri":  "file://" + dir,
					"name": "kinchaku",
				},
			},
		},
	})
	if respErr != nil || err != nil {
		log.Fatal(respErr, err)
	}
	//conn.SendRequest(ctx, "initialize", []byte("{\"capabilities\": {\"workspace\": { \"workspaceFolders\": true }}}"))
	//conn.SendRequest(ctx, "initialize", []byte("{\"capabilities\": {\"workspace\": { \"workspaceFolders\": true }}}"))
	log.Println("After initialize", response)
	_ = lsc.Initialized(&lsp.InitializedParams{})
	_ = lsc.WorkspaceDidChangeConfiguration(&lsp.DidChangeConfigurationParams{
		Settings: []byte("{\"intelephense.files.maxSize\": 3000000}"),
	})
	lsc.RegisterCustomNotification("indexingStarted", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		log.Println("indexingStarted", string(params))
	})
	lsc.RegisterCustomNotification("indexingEnded", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		log.Println("indexingEnded", string(params))
	})
}
