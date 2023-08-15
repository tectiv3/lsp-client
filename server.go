package main

import (
	"github.com/tectiv3/go-lsp"
	"go.bug.st/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

func (s *mateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := Panicf(recover(), "%v", r.Method); err != nil {
			LogError(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	Log("method: %s, length: %d %s", r.Method, r.ContentLength, r.URL.Path)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	decoder := json.NewDecoder(r.Body)
	mr := mateRequest{}
	err := decoder.Decode(&mr)
	if err != nil {
		LogError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resultChan := make(kvChan)
	var result *KeyValue
	tick := time.After(20 * time.Second)

	go s.processRequest(mr, resultChan)

	// block until result or timeout
	select {
	case <-tick:
		w.WriteHeader(http.StatusGatewayTimeout)
		w.Header().Set("Content-Type", "application/json")
		Log("method: %s Time out", mr.Method)
		json.NewEncoder(w).Encode(KeyValue{"result": "error", "message": "time out"})
		return
	case result = <-resultChan:
	}

	if result == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	tr, _ := json.Marshal(result)
	Log("method: %s %s", mr.Method, string(tr))
	json.NewEncoder(w).Encode(result)
}

func (s *mateServer) processRequest(mr mateRequest, cb kvChan) {
	defer s.handlePanic(mr)
	s.logger.LogIncomingRequest("", mr.Method, mr.Body)

	switch mr.Method {
	case "hover":
		//params := lsp.TextDocumentPositionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//	cb <- &KeyValue{"result": "error", "message": err.Error()}
		//	return
		//}
		//s.requestAndWait("textDocument/hover", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		resp := s.sendLSPRequest(s.intelephense, "textDocument/hover", params)

		cb <- &KeyValue{"result": "ok", "message": resp}
	case "completion":
		params := lsp.CompletionParams{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		//s.requestAndWait("textDocument/completion", params, cb)
	case "definition":
		params := lsp.TextDocumentPositionParams{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		//s.requestAndWait("textDocument/definition", params, cb)
	case "initialize":
		s.onInitialize(mr, cb)
	case "didOpen":
		s.onDidOpen(mr, cb)
	case "didClose":
		s.onDidClose(mr, cb)
	default:
		cb <- &KeyValue{"result": "error", "message": "unknown method"}
	}
	Log("method: %s %s", mr.Method, "processRequest finished")
}

func (s *mateServer) onDidOpen(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()

	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}
	//.URI.AsPath().String()
	fn := params.string("uri", "")
	if len(fn) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Invalid document uri"}
		return
	}

	go s.sendLSPRequest(s.intelephense, "textDocument/documentSymbol", params)

	if _, ok := s.openFiles[fn]; ok {
		s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
			"uri": fn,
		})
		time.Sleep(100 * time.Millisecond)
	}
	s.openFiles[fn] = time.Now()

	Log("waiting for diagnostics for %s", fn)
	resp := s.sendLSPRequest(s.intelephense, "textDocument/didOpen", params)

	cb <- &KeyValue{"result": "ok", "message": resp}
}

func (s *mateServer) onDidClose(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()

	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}
	fn := params.string("uri", "")
	if len(fn) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Invalid document uri"}
		return
	}
	s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
		"uri": fn,
	})
	delete(s.openFiles, fn)

	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) onInitialize(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()

	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}

	if s.initialized {
		cb <- &KeyValue{"result": "ok", "message": "already initialized"}
		return
	}

	s.sendLSPRequest(s.copilot, "initialize", KeyValue{})
	s.sendLSPRequest(s.copilot, "signIn", KeyValue{})

	dir := params.string("dir", "")
	if len(dir) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Empty dir"}
		return

	}
	s.sendLSPRequest(s.intelephense, "initialize", params)

	s.initialized = true
	s.currentWS = &workSpace{"event", ""}
	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) sendLSPRequest(out mrChan, method string, params KeyValue) *KeyValue {
	cb := make(kvChan)
	body, _ := json.Marshal(params)
	out <- &mateRequest{
		Method: method,
		Body:   body,
		CB:     cb,
	}

	if result := <-cb; result != nil {
		return result
	}

	return nil
}

func (s *mateServer) handlePanic(mr mateRequest) {
	if err := recover(); err != nil {
		Log("method: %s, bt: %s, Recovered from: %s", mr.Method, string(debug.Stack()), err)
	}
}

func startServer(intelephense, copilot mrChan, port string) {
	Log("Running webserver on port: %s", port)
	server := mateServer{
		intelephense: intelephense,
		copilot:      copilot, initialized: false,
		logger: &Logger{
			IncomingPrefix: "HTTP <-- IDE", OutgoingPrefix: "HTTP --> IDE",
			HiColor: hiGreenString, LoColor: greenString, ErrorColor: errorString,
		},
	}

	log.Fatal(http.ListenAndServe(":"+port, &server))
}
