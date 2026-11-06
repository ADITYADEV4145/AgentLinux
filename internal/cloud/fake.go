// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

// Fake is explicitly selected development infrastructure: it never creates a VM,
// executes a host command, or depicts a real Linux screen.
type Fake struct {
	mu         sync.Mutex
	resources  map[string]Resource
	generation map[string]string
	changes    map[string]int
}

func NewFake() *Fake {
	return &Fake{resources: map[string]Resource{}, generation: map[string]string{}, changes: map[string]int{}}
}
func (*Fake) Name() string    { return "fake-development" }
func (*Fake) Simulated() bool { return true }
func (p *Fake) Create(ctx context.Context, deployment string, s Session) (Resource, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := Resource{ID: "fake-" + s.ID, Session: s.ID, Deployment: deployment, State: "running"}
	p.resources[r.ID] = r
	p.generation[r.ID] = protocol.ID()
	return r, nil
}
func (p *Fake) List(context.Context, string) ([]Resource, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := []Resource{}
	for _, v := range p.resources {
		r = append(r, v)
	}
	return r, nil
}
func (p *Fake) Delete(ctx context.Context, r Resource) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.resources, r.ID)
	delete(p.generation, r.ID)
	delete(p.changes, r.ID)
	return nil
}
func (p *Fake) Probe(ctx context.Context, s Session) (Probe, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.resources[s.ProviderID]; !ok {
		return Probe{}, protocol.Fail("DISCONNECTED", "development resource absent")
	}
	c := protocol.SupportedCapabilities()
	c.GuestExec = false
	c.Apps = nil
	c.MaxExecMS = 0
	c.MaxOutputBytes = 0
	return Probe{true, p.generation[s.ProviderID], c}, nil
}
func (p *Fake) Call(ctx context.Context, s Session, q Operation) (Output, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.resources[s.ProviderID]; !ok {
		return Output{}, protocol.Fail("DISCONNECTED", "development resource absent")
	}
	g := p.generation[s.ProviderID]
	switch q.Method {
	case "input":
		p.changes[s.ProviderID]++
		return Output{Generation: g, Input: &protocol.InputResult{Dispatched: true, Note: "SIMULATED action; no computer was controlled"}}, nil
	case "screenshot":
		im := image.NewRGBA(image.Rect(0, 0, 1280, 800))
		draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{24, 32, 48, 255}), image.Point{}, draw.Src)
		shade := uint8(90 + p.changes[s.ProviderID]%100)
		draw.Draw(im, image.Rect(100, 300, 1180, 650), image.NewUniform(color.RGBA{shade, 30, 50, 255}), image.Point{}, draw.Src)
		// Original small bitmap glyphs make the synthetic nature visible in every frame.
		glyphs := map[rune][]string{'S': {"11111", "10000", "10000", "11111", "00001", "00001", "11111"}, 'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"}, 'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"}, 'U': {"10001", "10001", "10001", "10001", "10001", "10001", "11111"}, 'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"}, 'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"}, 'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"}, 'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"}, 'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"}}
		for n, ch := range "SIMULATED" {
			for y, row := range glyphs[ch] {
				for x, v := range row {
					if v == '1' {
						draw.Draw(im, image.Rect(100+n*120+x*18, 100+y*18, 118+n*120+x*18, 118+y*18), image.White, image.Point{}, draw.Src)
					}
				}
			}
		}
		var b bytes.Buffer
		_ = png.Encode(&b, im)
		return Output{PNG: b.Bytes(), Generation: g}, nil
	default:
		return Output{}, protocol.Fail("UNSUPPORTED_OPERATION", "development fake has no command/app/file implementation")
	}
}
