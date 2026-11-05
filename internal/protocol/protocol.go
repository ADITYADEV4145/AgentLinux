// SPDX-License-Identifier: Apache-2.0

// Package protocol defines the bounded guest control protocol, independent of MCP.
package protocol

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const Version = "2.0"
const GuestVersion = "0.3.5"
const MaxFrame = 16 << 20
const MaxRequest = 256 << 10
const MaxPNG = 8 << 20

type Request struct {
	Version   string       `json:"version"`
	ID        string       `json:"id"`
	MachineID string       `json:"machineId"`
	SessionID string       `json:"sessionId,omitempty"`
	Method    string       `json:"method"`
	Nonce     string       `json:"nonce,omitempty"`
	Input     *Input       `json:"input,omitempty"`
	Exec      *ExecRequest `json:"exec,omitempty"`
	App       string       `json:"app,omitempty"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Failure) Error() string      { return e.Code + ": " + e.Message }
func Fail(code, message string) error { return &Failure{code, message} }

type Info struct {
	GuestVersion string         `json:"guestVersion"`
	Protocol     string         `json:"protocol"`
	MachineID    string         `json:"machineId"`
	SessionID    string         `json:"sessionId"`
	BootID       string         `json:"bootId"`
	OS           string         `json:"os"`
	Arch         string         `json:"arch"`
	Uptime       string         `json:"uptime,omitempty"`
	NonceSHA256  string         `json:"nonceSha256,omitempty"`
	Operations   []string       `json:"operations"`
	Capabilities *Capabilities  `json:"capabilities,omitempty"`
	Desktop      *DesktopStatus `json:"desktop,omitempty"`
	Frame        *Frame         `json:"frame,omitempty"`
	Execution    *Execution     `json:"execution,omitempty"`
	InputResult  *InputResult   `json:"inputResult,omitempty"`
}
type Response struct {
	Version string   `json:"version"`
	ID      string   `json:"id"`
	Result  *Info    `json:"result,omitempty"`
	Error   *Failure `json:"error,omitempty"`
}

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func ValidID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}
func Read(r *bufio.Reader, v any) error {
	line, err := r.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) || len(line) > MaxFrame {
		return Fail("FRAME_TOO_LARGE", "message exceeds negotiated framing limit; connection closed")
	}
	if err != nil {
		return err
	}
	if !utf8.Valid(line) {
		return Fail("INVALID_MESSAGE", "invalid UTF-8")
	}
	// Reject duplicate keys at every level before schema decoding.
	d := json.NewDecoder(bytes.NewReader(line))
	if err = unique(d); err != nil {
		return Fail("INVALID_MESSAGE", err.Error())
	}
	if _, err = d.Token(); err != io.EOF {
		return Fail("INVALID_MESSAGE", "trailing JSON data")
	}
	d = json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return Fail("INVALID_MESSAGE", "invalid message schema")
	}
	return nil
}
func unique(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return fmt.Errorf("duplicate or invalid object key")
			}
			seen[s] = true
			if e = unique(d); e != nil {
				return e
			}
		}
		_, e = d.Token()
		return e
	case json.Delim('['):
		for d.More() {
			if e = unique(d); e != nil {
				return e
			}
		}
		_, e = d.Token()
		return e
	}
	return nil
}
func RequestReader(r io.Reader) *bufio.Reader { return bufio.NewReaderSize(r, MaxRequest) }

func Reader(r io.Reader) *bufio.Reader { return bufio.NewReaderSize(r, MaxFrame+1) }
func Write(w io.Writer, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(b)+1 > MaxFrame {
		return Fail("FRAME_TOO_LARGE", "outgoing message exceeds limit")
	}
	b = append(b, '\n')
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

// WriteRequest enforces the smaller request budget before any mutation is sent.
func WriteRequest(w io.Writer, q Request) error {
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	if len(b)+1 > MaxRequest {
		return Fail("REQUEST_TOO_LARGE", "request exceeds 256 KiB")
	}
	return Write(w, q)
}

// Phase 3.5 extends launch targets without changing the v2 wire contract.
func CompatibleGuestVersion(v string) bool { return v == GuestVersion || v == "0.2.0" }
