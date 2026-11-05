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

const Version = "1.0"
const GuestVersion = "0.1.0"
const MaxFrame = 16 << 10

type Request struct {
	Version   string `json:"version"`
	ID        string `json:"id"`
	MachineID string `json:"machineId"`
	SessionID string `json:"sessionId,omitempty"`
	Method    string `json:"method"`
	Nonce     string `json:"nonce,omitempty"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Failure) Error() string      { return e.Code + ": " + e.Message }
func Fail(code, message string) error { return &Failure{code, message} }

type Info struct {
	GuestVersion string   `json:"guestVersion"`
	Protocol     string   `json:"protocol"`
	MachineID    string   `json:"machineId"`
	SessionID    string   `json:"sessionId"`
	BootID       string   `json:"bootId"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Uptime       string   `json:"uptime,omitempty"`
	NonceSHA256  string   `json:"nonceSha256,omitempty"`
	Operations   []string `json:"operations"`
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
		return Fail("FRAME_TOO_LARGE", "guest message exceeds 16 KiB; connection closed")
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
