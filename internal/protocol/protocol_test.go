// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestFrames(t *testing.T) {
	q := Request{Version: Version, ID: ID(), MachineID: ID(), Method: "hello"}
	var b bytes.Buffer
	if e := Write(&b, q); e != nil {
		t.Fatal(e)
	}
	var got Request
	if e := Read(Reader(&b), &got); e != nil || got != q {
		t.Fatalf("round trip: %#v %v", got, e)
	}
	for name, input := range map[string]string{"duplicate": `{"id":"a","id":"b"}` + "\n", "unknown": `{"command":"rm"}` + "\n", "malformed": "{bad}\n", "oversized": strings.Repeat("a", MaxFrame+1) + "\n", "invalid UTF8": string([]byte{0xff, '\n'}), "trailing": `{} {}` + "\n"} {
		t.Run(name, func(t *testing.T) {
			var r Request
			if Read(Reader(strings.NewReader(input)), &r) == nil {
				t.Fatal("accepted invalid frame")
			}
		})
	}
	if e := Read(Reader(strings.NewReader("{}")), &got); !errors.Is(e, io.EOF) {
		t.Fatalf("partial frame must fail: %v", e)
	}
}
