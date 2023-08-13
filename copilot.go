package main

import (
	"context"
	"encoding/json"
	"go.bug.st/lsp"
	"log"
	"os"
	"os/exec"
	"time"
)

func startCopilot(dir string) {
	cmd := exec.Command("/opt/homebrew/opt/node@16/bin/node", "/opt/homebrew/bin/copilot-node-server")

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

	time.Sleep(1 * time.Second)
	conn := lsc.GetConnection()

	conn.SendRequest(ctx, "initialize", []byte("{\"capabilities\": {\"workspace\": { \"workspaceFolders\": true }}}"))
	log.Println("After initialize")
	lsc.Initialized(&lsp.InitializedParams{})
	time.Sleep(1 * time.Second)
	conn.SendRequest(ctx, "setEditorInfo", []byte("{\"editorInfo\": {\"name\": \"Textmate\", \"version\": \"2.0.23\"}, \"editorPluginInfo\": {\"name\": \"lsp-bridge\", \"version\": \"0.0.1\"}}"))
	time.Sleep(1 * time.Second)

	resp, respErr, err := conn.SendRequest(ctx, "signInInitiate", []byte("{\"dummy\": \"signInInitiate\"}"))
	if err != nil || respErr != nil {
		log.Fatal(respErr, err)
	}
	if string(resp) == "null" {
		log.Fatal("Empty response")
	}
	var res signInResponse
	json.Unmarshal(resp, &res)

	//        eval_in_emacs("browse-url", result['verificationUri'])
	//        message_emacs(f'Please enter user-code {result["userCode"]}')
	log.Println(res.Status)

	defer stdin.Close()
	cmd.Wait()
}
