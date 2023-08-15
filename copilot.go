package main

import (
	"context"
	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"log"
	"os"
	"os/exec"
)

func startCopilot(in mrChan) {
	cmd := exec.Command("/opt/homebrew/opt/node@16/bin/node", "/opt/homebrew/bin/copilot-node-server")

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	lsc := lsp.NewClient(stdout, stdin, cmdHandler{}, func(err error) {
		LogError(err)
	})
	lsc.SetLogger(&Logger{
		IncomingPrefix: "LS <-- Copilot", OutgoingPrefix: "LS --> Copilot",
		HiColor: hiGreenString, LoColor: greenString, ErrorColor: errorString,
	})
	go lsc.Run()

	lsc.RegisterCustomNotification("statusNotification", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		logger.Logf("statusNotification %s", string(params))
	})

	go processCopilotRequests(in, lsc)

	defer stdin.Close()
	cmd.Wait()
}

func processCopilotRequests(in mrChan, lsc *lsp.Client) {
	Log("Waiting for input")
	ctx := context.Background()
	conn := lsc.GetConnection()
	for {
		request := <-in
		Log("LS <-- IDE %s %s %s", "request", request.Method, string(request.Body))

		switch request.Method {
		case "initialize":
			sendRequest("initialize", KeyValue{
				"capabilities": KeyValue{"workspace": KeyValue{"workspaceFolders": true}},
			}, conn, ctx)
			log.Println("After initialize")
			lsc.Initialized(&lsp.InitializedParams{})
			sendRequest("setEditorInfo", KeyValue{
				"editorInfo":       KeyValue{"name": "Textmate", "version": "2.0.23"},
				"editorPluginInfo": KeyValue{"name": "lsp-bridge", "version": "0.0.1"},
			}, conn, ctx)
			request.CB <- &KeyValue{"status": "ok"}
		case "signIn":
			resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
			var res signInResponse
			json.Unmarshal(resp, &res)
			//        eval_in_emacs("browse-url", result['verificationUri'])
			//        message_emacs(f'Please enter user-code {result["userCode"]}')
			//log.Println(res.Status)
			request.CB <- &KeyValue{"status": res.Status}
		case "getCompletions":
			resp := sendRequest("getCompletions", KeyValue{
				"doc": KeyValue{
					"source":       "func main() {\n\t// print hello world message\n\n}",
					"tabSize":      2,
					"indentSize":   2,
					"insertSpaces": false,
					"version":      0,
					"path":         "/tmp/test.go",
					"uri":          "file:///tmp/test.go",
					"relativePath": "test.go",
					"languageId":   "go",
					"position": KeyValue{
						"line":      3,
						"character": 0,
					},
				},
			}, conn, ctx)

			request.CB <- &KeyValue{"status": "ok", "result": string(resp)}
		case "getCompletionsCycling":
			//
		}
	}
}

func sendRequest(method string, request KeyValue, conn *jsonrpc.Connection, ctx context.Context) json.RawMessage {
	body, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return []byte{}
	}

	resp, respErr, err := conn.SendRequest(ctx, method, body)
	if err != nil || respErr != nil {
		log.Println(respErr, err)
		return []byte{}
	}
	if string(resp) == "null" {
		log.Println("Empty response")
		return []byte{}
	}

	log.Println(method, string(resp))
	return resp
}
