// SPDX-License-Identifier: Apache-2.0
package viewer

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strings"
)

const ImageID = 4145

// Size uses actual terminal pixels, never a guessed cell aspect ratio.
type Size struct{ Columns, Rows, PixelWidth, PixelHeight int }
type Placement struct{ Column, Row, Columns, Rows int }

func Fit(s Size) Placement {
	if s.Columns < 20 || s.Rows < 6 || s.PixelWidth <= 0 || s.PixelHeight <= 0 {
		return Placement{}
	}
	cw, ch := float64(s.PixelWidth)/float64(s.Columns), float64(s.PixelHeight)/float64(s.Rows)
	cols := int(math.Floor(math.Min(float64(s.PixelWidth), float64(s.Rows-2)*ch*1280/800) / cw))
	rows := int(math.Ceil(float64(cols) * cw * 800 / 1280 / ch))
	if cols < 1 || rows < 1 || rows > s.Rows-2 {
		return Placement{}
	}
	return Placement{Column: 1 + (s.Columns-cols)/2, Row: 2 + (s.Rows-2-rows)/2, Columns: cols, Rows: rows}
}
func Candidate(tty bool, term, program, kitty, tmux, screen string) bool {
	return tty && tmux == "" && screen == "" && (term == "xterm-kitty" || program == "kitty" || kitty != "")
}

const Query = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\\x1b[c"

func Supported(reply []byte) bool { return strings.Contains(string(reply), "\x1b_Gi=31;OK\x1b\\") }
func deleteID(id int) string      { return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", id) }
func DeleteImage() string         { return deleteID(ImageID) + deleteID(ImageID+1) }

type WriteFunc func(context.Context, []byte) error

// Renderer keeps at most two terminal images. Transmit the replacement before
// placing it, and free the old image only after the new placement is submitted.
type Renderer struct{ current int }

func Render(ctx context.Context, write WriteFunc, p Placement, png []byte) error {
	var r Renderer
	return r.Render(ctx, write, p, png)
}
func (r *Renderer) Render(ctx context.Context, write WriteFunc, p Placement, png []byte) error {
	if p.Columns == 0 {
		return nil
	}
	id := ImageID
	if r.current == ImageID {
		id++
	}
	encoded := base64.StdEncoding.EncodeToString(png)
	for first := true; len(encoded) > 0; first = false {
		n := min(4096, len(encoded))
		more := 0
		if n < len(encoded) {
			more = 1
		}
		header := fmt.Sprintf("\x1b_Gm=%d;", more)
		if first {
			header = fmt.Sprintf("\x1b_Ga=t,t=d,f=100,i=%d,q=2,m=%d;", id, more)
		}
		if e := write(ctx, []byte(header+encoded[:n]+"\x1b\\")); e != nil {
			return e
		}
		encoded = encoded[n:]
	}
	placement := fmt.Sprintf("\x1b[%d;%dH\x1b_Ga=p,i=%d,c=%d,C=1,q=2;\x1b\\", p.Row, p.Column, id, p.Columns)
	if e := write(ctx, []byte(placement)); e != nil {
		return e
	}
	if r.current != 0 {
		if e := write(ctx, []byte(deleteID(r.current))); e != nil {
			return e
		}
	}
	r.current = id
	return nil
}
