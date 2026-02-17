package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

var vueClient *handler

func startVue(in mrChan) {
	if len(config.VueLSPath) == 0 {
		Log("Vue LS path not set")
		go func() {
			for {
				request := <-in
				request.CB <- &KeyValue{"status": "ok"}
			}
		}()
		return
	}
	vueClient = startRPCServer("vue", config.NodePath, config.VueLSPath, "--stdio", "--tsdk="+config.TsdkPath)
	vueClient.Requests = make(map[string]string)
	vueClient.lsc.SetLogger(&Logger{
		IncomingPrefix: "LSV <-- Vue", OutgoingPrefix: "LSV --> Vue",
		HiColor: hiBlueString, LoColor: blueString, ErrorColor: errorString,
	})
	vueClient.lsc.RegisterCustomNotification("indexingStarted", func(jsonrpc.FunctionLogger, json.RawMessage) {})
	vueClient.lsc.RegisterCustomNotification("indexingEnded", func(jsonrpc.FunctionLogger, json.RawMessage) {})

	go vueClient.lsc.Run()
	go vueClient.processVueRequests(in)
}

func (c *handler) processVueRequests(in mrChan) {
	defer catchAndLogPanic(func() {
		c.processVueRequests(in)
	})

	Log("Vue LS is waiting for input")
	ctx := context.Background()
	lsc := c.lsc

	for {
		request := <-in
		Log("LSV <-- IDE %s %s %db", "request", request.Method, len(string(request.Body)))

		switch request.Method {
		case "initialize":
			pid := os.Getpid()
			var params KeyValue
			if err := json.Unmarshal(request.Body, &params); err != nil {
				LogError(err)
				request.CB <- &KeyValue{"result": "error", "message": "empty dir"}
				continue
			}
			dir := params.string("dir", "")
			name := params.string("name", "vueProject")
			var folders []lsp.WorkspaceFolder
			paramFolders := params.array("folders", []interface{}{})
			if len(paramFolders) > 0 {
				Log("folders: %d, %v", len(paramFolders), paramFolders)
				for _, f := range paramFolders {
					if m, ok := f.(map[string]interface{}); ok {
						folder := KeyValue(m)
						uri, _ := lsp.NewDocumentURIFromURL(folder.string("uri", ""))
						folders = append(folders, lsp.WorkspaceFolder{
							URI:  uri,
							Name: folder.string("name", ""),
						})
					} else {
						panic("whaaaat")
					}

				}
			} else {
				folders = append(folders, lsp.WorkspaceFolder{
					URI:  lsp.NewDocumentURI(dir),
					Name: name,
				})

			}

			ctxC, cancel := context.WithTimeout(ctx, time.Second)
			_, respErr, err := lsc.Initialize(ctxC, &lsp.InitializeParams{
				ProcessID: &pid,
				InitializationOptions: lsp.KeyValue{
					"vue": lsp.KeyValue{"hybridMode": false},
				},
				Capabilities: lsp.KeyValue{
					"workspace":        KeyValue{"workspaceFolders": true},
					"workspaceFolders": folders,
				},
				WorkspaceFolders: &folders,
			})
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "initialize error"}
				cancel()
				continue
			}
			cancel()
			go lsc.Initialized(&lsp.InitializedParams{})

			request.CB <- &KeyValue{"status": "ok"}
		case "textDocument/hover":
			params := lsp.TextDocumentPositionParams{}
			if err := json.Unmarshal(request.Body, &params); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				continue
			}
			response, respErr, err := lsc.TextDocumentHover(ctx, &lsp.HoverParams{TextDocumentPositionParams: params})
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "hover error"}
				continue
			}
			request.CB <- &KeyValue{"status": "ok", "result": response}
		case "didChangeWorkspaceFolders":
			folder := lsp.WorkspaceFolder{}
			if err := json.Unmarshal(request.Body, &folder); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				continue
			}
			lsc.WorkspaceDidChangeWorkspaceFolders(&lsp.DidChangeWorkspaceFoldersParams{
				Event: lsp.WorkspaceFoldersChangeEvent{
					Added:   []lsp.WorkspaceFolder{folder},
					Removed: []lsp.WorkspaceFolder{},
				},
			})
		case "textDocument/definition":
			fallthrough
		case "textDocument/completion":
			response, respErr, err := lsc.GetConnection().SendRequest(ctx, request.Method, request.Body)
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "hover error"}
				continue
			}
			request.CB <- &KeyValue{"status": "ok", "result": response}
		case "textDocument/documentSymbol":
			lsc.GetConnection().SendRequest(ctx, request.Method, request.Body)

			var params KeyValue
			if err := json.Unmarshal(request.Body, &params); err != nil {
				LogError(err)
			}
			uuid := params.string("uuid", "")
			if len(uuid) != 0 {
				request.CB <- &KeyValue{"status": "ok"}
			}

			go func() {
				if config.EnableLogging {
					Log("Waiting for diagnostics")
				}
				c.Lock()
				c.waitingForDiagnostics = true
				c.Unlock()

				diagnostics := <-lsc.GetHandler().GetDiagnosticChannel()
				c.Lock()
				c.waitingForDiagnostics = false
				c.Unlock()

				if len(config.MatePath) != 0 {
					applyTextmateMarks(uuid, diagnostics)
				} else {
					request.CB <- &KeyValue{"status": "ok", "result": diagnostics.Diagnostics}
				}
			}()
		case "textDocument/didOpen":
			textDocument := &KeyValue{}
			if err := json.Unmarshal(request.Body, textDocument); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				return
			}
			uri, _ := lsp.NewDocumentURIFromURL(textDocument.string("uri", ""))
			go lsc.TextDocumentDidOpen(&lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{
				URI:        uri,
				LanguageID: textDocument.string("languageId", ""),
				Version:    textDocument.int("version", 0),
				Text:       textDocument.string("text", ""),
			}})
			request.CB <- &KeyValue{"status": "ok"}
		case "textDocument/didClose":
			textDocument := lsp.TextDocumentIdentifier{}
			if err := json.Unmarshal(request.Body, &textDocument); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				return
			}
			go lsc.TextDocumentDidClose(&lsp.DidCloseTextDocumentParams{TextDocument: textDocument})
			request.CB <- &KeyValue{"status": "ok"}
		}
	}
}
