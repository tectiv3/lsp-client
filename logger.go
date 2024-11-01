// This file is part of arduino-language-server.
//
// Copyright 2022 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU Affero General Public License version 3,
// which covers the main part of arduino-language-server.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/agpl-3.0.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

// Logger is a lsp logger
type Logger struct {
	IncomingPrefix, OutgoingPrefix string
	HiColor, LoColor               func(format string, a ...interface{}) string
	ErrorColor                     func(format string, a ...interface{}) string
}

func init() {
	log.SetFlags(log.LstdFlags)
}

// LogOutgoingRequest prints an outgoing request into the log
func (l *Logger) LogOutgoingRequest(id string, method string, params json.RawMessage) {
	if config.EnableLogging {
		log.Print(l.decorate(l.HiColor("%s REQU %s %s %s", l.OutgoingPrefix, method, id, string(params))))
	}
}

// LogOutgoingCancelRequest prints an outgoing cancel request into the log
func (l *Logger) LogOutgoingCancelRequest(id string) {
	if config.EnableLogging {
		log.Print(l.LoColor("%s CANCEL %s", l.OutgoingPrefix, id))
	}
}

// LogIncomingResponse prints an incoming response into the log if there is no error
func (l *Logger) LogIncomingResponse(id string, method string, resp json.RawMessage, respErr *jsonrpc.ResponseError) {
	e := ""
	if respErr != nil {
		e = l.ErrorColor(" ERROR: %s", respErr.AsError())
	} else {
		e = string(resp)
	}
	if config.EnableLogging {
		log.Print(l.decorate(l.LoColor("%s RESP %s %s %s", l.IncomingPrefix, method, id, e)))
	}
}

// LogOutgoingNotification prints an outgoing notification into the log
func (l *Logger) LogOutgoingNotification(method string, params json.RawMessage) {
	if config.EnableLogging {
		log.Print(l.decorate(l.HiColor("%s NOTIF %s", l.OutgoingPrefix, method)))
	}
}

// LogIncomingRequest prints an incoming request into the log
func (l *Logger) LogIncomingRequest(id string, method string, params json.RawMessage) jsonrpc.FunctionLogger {
	spaces := "                                               "

	if config.EnableLogging {
		log.Print(l.decorate(l.HiColor(fmt.Sprintf("%s REQU %s %s", l.IncomingPrefix, method, id))))
	}

	return &FunctionLogger{
		colorFunc: l.HiColor,
		prefix:    fmt.Sprintf("%s      %s %s", spaces[:len(l.IncomingPrefix)], method, id),
	}
}

// LogIncomingCancelRequest prints an incoming cancel request into the log
func (l *Logger) LogIncomingCancelRequest(id string) {
	if config.EnableLogging {
		log.Print(l.LoColor("%s CANCEL %s", l.IncomingPrefix, id))
	}
}

// LogOutgoingResponse prints an outgoing response into the log if there is no error
func (l *Logger) LogOutgoingResponse(id string, method string, resp json.RawMessage, respErr *jsonrpc.ResponseError) {
	e := ""
	if respErr != nil {
		e = l.ErrorColor(" ERROR: %s", respErr.AsError())
	}
	if config.EnableLogging {
		log.Print(l.decorate(l.LoColor("%s RESP %s %s %s", l.OutgoingPrefix, method, id, e)))
	}
}

// LogIncomingNotification prints an incoming notification into the log
func (l *Logger) LogIncomingNotification(method string, params json.RawMessage) jsonrpc.FunctionLogger {
	spaces := "                                               "
	return &FunctionLogger{
		colorFunc: l.HiColor,
		prefix:    fmt.Sprintf("%s       %s", spaces[:len(l.IncomingPrefix)], method),
	}
}

// LogIncomingDataDelay prints the delay of incoming data into the log
func (l *Logger) LogIncomingDataDelay(delay time.Duration) {
	//log.Printf("IN Elapsed: %v", delay)
}

// LogOutgoingDataDelay prints the delay of outgoing data into the log
func (l *Logger) LogOutgoingDataDelay(delay time.Duration) {
	//log.Printf("OUT Elapsed: %v", delay)
}

func (l *Logger) decorate(format string) string {
	file_info := ""
	pc, file, line, ok := runtime.Caller(2)
	if ok {
		// Get caller function name.
		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn == nil {
			fnName = "?()"
		} else {
			fnName = strings.TrimLeft(filepath.Ext(fn.Name()), ".")
		}
		fileName := filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file))

		file_info = fmt.Sprintf("[%s:%d %s] ", fileName, line, fnName)
	}

	return file_info + format
}

// FunctionLogger is a lsp function logger
type FunctionLogger struct {
	colorFunc func(format string, a ...interface{}) string
	prefix    string
}

// NewLSPFunctionLogger creates a new function logger
func NewLSPFunctionLogger(colorFunction func(format string, a ...interface{}) string, prefix string) *FunctionLogger {

	return &FunctionLogger{
		colorFunc: colorFunction,
		prefix:    prefix,
	}
}

// Foreground Hi-Intensity text colors
const (
	FgHiBlack int = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Foreground text colors
const (
	FgBlack int = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack int = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

const escape = "\x1b"

const (
	Reset int = iota
)

func Log(format string, a ...interface{}) {
	logger.Logf(format, a...)
}

func LogError(err error) {
	logger.Logf(errorString("Error: %v", err))
}

// Logf logs the given message
func (l *FunctionLogger) Logf(format string, a ...interface{}) {
	log.Print(l.decorate(l.colorFunc(l.prefix+": "+format, a...)))
}

func (l *FunctionLogger) decorate(format string) string {
	file_info := ""
	pc, file, line, ok := runtime.Caller(3)
	if ok {
		// Get caller function name.
		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn == nil {
			fnName = "?()"
		} else {
			fnName = strings.TrimLeft(filepath.Ext(fn.Name()), ".")
		}
		fileName := filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file))

		file_info = fmt.Sprintf("[%s:%d %s] ", fileName, line, fnName)
	}

	return file_info + format
}

func c_format(colors ...int) string {
	return fmt.Sprintf("%s[%sm", escape, c_sequence(colors...))
}

func c_unformat() string {
	return fmt.Sprintf("%s[%dm", escape, Reset)
}

func c_sequence(colors ...int) string {
	format := make([]string, len(colors))
	for i, v := range colors {
		format[i] = strconv.Itoa(v)
	}

	return strings.Join(format, ";")

}

func hiRedString(format string, a ...interface{}) string {
	return colorFormat(format, FgHiRed, a...)
}
func hiGreenString(format string, a ...interface{}) string {
	return colorFormat(format, FgHiGreen, a...)
}
func hiMagentaString(format string, a ...interface{}) string {
	return colorFormat(format, FgHiMagenta, a...)
}
func hiBlueString(format string, a ...interface{}) string {
	return colorFormat(format, FgHiBlue, a...)
}
func redString(format string, a ...interface{}) string {
	return colorFormat(format, FgRed, a...)
}
func greenString(format string, a ...interface{}) string {
	return colorFormat(format, FgGreen, a...)
}
func blueString(format string, a ...interface{}) string {
	return colorFormat(format, FgBlue, a...)
}
func colorFormat(format string, color int, a ...interface{}) string {
	return c_format(color) + fmt.Sprintf(format, a...) + c_unformat()
}
func errorString(format string, a ...interface{}) string {
	return c_format(BgHiMagenta, FgHiWhite) + fmt.Sprintf(format, a...) + c_unformat()
}
