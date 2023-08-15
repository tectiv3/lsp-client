package main

import (
	"database/sql/driver"
	"fmt"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

type ClientInfo struct {
	Name    string  `json:"name,required"`
	Version *string `json:"version,omitempty"`
}

type Workspace struct {
	WorkspaceFolders bool `json:"workspaceFolders,omitempty"`
}

type signInResponse struct {
	Status          string `json:"status"`
	User            string `json:"user"`
	VerificationUri string `json:"verificationUri,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
}

// KeyValue is basic key:value struct
type KeyValue map[string]interface{}

// bool returns the value of the given name, assuming the value is a boolean.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) bool(name string, defaultValue bool) bool {
	if v, found := kv[name]; found {
		if castValue, is := v.(bool); is {
			return castValue
		}
	}
	return defaultValue
}

// int returns the value of the given name, assuming the value is an int.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) int(name string, defaultValue int) int {
	if v, found := kv[name]; found {
		if castValue, is := v.(int); is {
			return castValue
		}
	}
	return defaultValue
}

// string returns the value of the given name, assuming the value is a string.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) string(name string, defaultValue string) string {
	if v, found := kv[name]; found {
		if castValue, is := v.(string); is {
			return castValue
		}
	}
	return defaultValue
}

// float64 returns the value of the given name, assuming the value is a float64.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) float64(name string, defaultValue float64) float64 {
	if v, found := kv[name]; found {
		if castValue, is := v.(float64); is {
			return castValue
		}
	}
	return defaultValue
}

// Value get value of KeyValue
func (kv KeyValue) Value() (driver.Value, error) {
	return json.Marshal(kv)
}

// Scan scan value into KeyValue
func (kv *KeyValue) Scan(value interface{}) error {
	bytes, ok := value.([]byte)

	if !ok {
		return fmt.Errorf("failed to unmarshal JSON value: %v", value)
	}

	return json.Unmarshal(bytes, kv)
}

type mateRequest struct {
	Method string
	Body   json.RawMessage
	CB     kvChan
}

type mateServer struct {
	copilot      mrChan
	intelephense mrChan
	initialized  bool
	logger       jsonrpc.Logger
	openFiles    map[string]time.Time
	currentWS    *workSpace
	openFolders  []string
	sync.Mutex
}

type workSpace struct {
	name string
	uri  string
}

type kvChan chan *KeyValue
type mrChan chan *mateRequest

type DocumentURI string

type TextDocumentItem struct {
	/**
	 * The text document's URI.
	 */
	URI DocumentURI `json:"uri"`
	/**
	 * The text document's language identifier.
	 */
	LanguageID string `json:"languageId"`
	/**
	 * The version number of this document (it will strictly increase after each
	 * change, including undo/redo).
	 */
	Version int `json:"version"`
	/**
	 * The content of the opened text document.
	 */
	Text string `json:"text"`
}

// NewReadWriteCloser create an io.ReadWriteCloser from given io.ReadCloser and io.WriteCloser.
func NewReadWriteCloser(in io.ReadCloser, out io.WriteCloser) io.ReadWriteCloser {
	return &combinedReadWriteCloser{in, out}
}

// OpenLogFileAs creates a log file in GlobalLogDirectory.
func openLogFileAs(filename string) *os.File {
	path := "./logs/" + filename
	res, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %s", err)
	}
	res.WriteString("\n\n\nStarted logging.\n")

	return res
}

func LogReadWriteCloserAs(upstream io.ReadWriteCloser, filename string) io.ReadWriteCloser {
	return &dumper{
		upstream: upstream,
		logfile:  openLogFileAs(filename),
	}
}

type dumper struct {
	upstream io.ReadWriteCloser
	logfile  *os.File
	reading  bool
	writing  bool
}

func (d *dumper) Read(buff []byte) (int, error) {
	n, err := d.upstream.Read(buff)
	if err != nil {
		d.logfile.Write([]byte(fmt.Sprintf("<<< Read Error: %s\n", err)))
	} else {
		if !d.reading {
			d.reading = true
			d.writing = false
			d.logfile.Write([]byte("\n<<<\n"))
		}
		d.logfile.Write(buff[:n])
	}
	return n, err
}

func (d *dumper) Write(buff []byte) (int, error) {
	n, err := d.upstream.Write(buff)
	if err != nil {
		_, _ = d.logfile.Write([]byte(fmt.Sprintf(">>> Write Error: %s\n", err)))
	} else {
		if !d.writing {
			d.writing = true
			d.reading = false
			d.logfile.Write([]byte("\n>>>\n"))
		}
		_, _ = d.logfile.Write(buff[:n])
	}
	return n, err
}

func (d *dumper) Close() error {
	err := d.upstream.Close()
	_, _ = d.logfile.Write([]byte(fmt.Sprintf("--- Stream closed, err=%s\n", err)))
	_ = d.logfile.Close()
	return err
}

type combinedReadWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (sd *combinedReadWriteCloser) Read(p []byte) (int, error) {
	return sd.reader.Read(p)
}

func (sd *combinedReadWriteCloser) Write(p []byte) (int, error) {
	return sd.writer.Write(p)
}

func (sd *combinedReadWriteCloser) Close() error {
	ierr := sd.reader.Close()
	oerr := sd.writer.Close()
	if ierr != nil {
		return ierr
	}
	if oerr != nil {
		return oerr
	}
	return nil
}
