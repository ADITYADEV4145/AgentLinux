// SPDX-License-Identifier: Apache-2.0
// Package viewer provides bounded, view-only consumers of the common machine interface.
package viewer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/machine"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
)

type Snapshot struct {
	Frame    *protocol.Frame
	Digest   string
	Observed time.Time
}
type Metrics struct {
	Captures      uint64    `json:"captureAttempts"`
	Frames        uint64    `json:"validFrames"`
	Changes       uint64    `json:"changedFrames"`
	Errors        uint64    `json:"captureErrors"`
	CapturedBytes uint64    `json:"capturedPNGBytes"`
	SentBytes     uint64    `json:"displayPNGBytes"`
	CaptureMS     []float64 `json:"recentCaptureMilliseconds"`
}
type Hub struct {
	Machine   machine.Machine
	Interval  time.Duration
	mu        sync.Mutex
	gate      chan struct{}
	last      Snapshot
	err       error
	next      time.Time
	unchanged int
	stats     Metrics
	revision  string
}

func NewHub(m machine.Machine, interval time.Duration) *Hub {
	return &Hub{Machine: m, Interval: interval, gate: make(chan struct{}, 1)}
}

// Next performs no background work. At most one bounded capture and one cached
// frame exist, regardless of client polling rate. Idle pages/viewers do no captures.
func (h *Hub) Next(ctx context.Context) (Snapshot, error) {
	revision := ""
	if source, ok := h.Machine.(interface{ ObservationRevision() string }); ok {
		revision = source.ObservationRevision()
	}
	h.mu.Lock()
	if revision != "" && revision != h.revision {
		h.revision = revision
		h.next = time.Time{}
		h.unchanged = 0
	}
	if time.Now().Before(h.next) {
		s, e := h.last, h.err
		h.mu.Unlock()
		return s, e
	}
	h.mu.Unlock()
	select {
	case h.gate <- struct{}{}:
		defer func() { <-h.gate }()
	default:
		return Snapshot{}, protocol.Fail("MACHINE_BUSY", "capture already in progress")
	}
	h.mu.Lock()
	if time.Now().Before(h.next) {
		s, e := h.last, h.err
		h.mu.Unlock()
		return s, e
	}
	h.mu.Unlock()
	c, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	start := time.Now()
	frame, e := h.Machine.Screenshot(c)
	if e == nil {
		if frame == nil {
			e = protocol.Fail("INVALID_FRAME", "no frame")
		} else {
			e = frame.Validate()
			if len(frame.Generation) > 128 {
				e = protocol.Fail("INVALID_FRAME", "oversized generation")
			}
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.Captures++
	h.stats.CaptureMS = append(h.stats.CaptureMS, float64(time.Since(start).Microseconds())/1000)
	if len(h.stats.CaptureMS) > 256 {
		copy(h.stats.CaptureMS, h.stats.CaptureMS[len(h.stats.CaptureMS)-256:])
		h.stats.CaptureMS = h.stats.CaptureMS[:256]
	}
	delay := h.Interval
	if delay < 250*time.Millisecond {
		delay = 250 * time.Millisecond
	}
	if e != nil {
		h.stats.Errors++
		h.err = e
		h.next = time.Now().Add(delay)
		return Snapshot{}, e
	}
	sum := sha256.Sum256(frame.PNG)
	digest := hex.EncodeToString(sum[:])
	h.stats.Frames++
	h.stats.CapturedBytes += uint64(len(frame.PNG))
	if digest == h.last.Digest {
		h.unchanged++
	} else {
		h.unchanged = 0
		h.stats.Changes++
	}
	// Static screens slow down to at most one capture every five seconds.
	if h.unchanged >= 3 {
		delay *= time.Duration(1 + h.unchanged/3)
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
	}
	h.last = Snapshot{Frame: frame, Digest: digest, Observed: time.Now()}
	h.err = nil
	h.next = time.Now().Add(delay)
	return h.last, nil
}
func (h *Hub) Sent(n int) { h.mu.Lock(); defer h.mu.Unlock(); h.stats.SentBytes += uint64(n) }
func (h *Hub) Metrics() Metrics {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.stats
	r.CaptureMS = append([]float64(nil), r.CaptureMS...)
	return r
}

// WaitIdle confirms the last canceled capture has returned, including owned
// backend subprocess cleanup. Call after closing network request contexts.
func (h *Hub) WaitIdle(ctx context.Context) error {
	select {
	case h.gate <- struct{}{}:
		<-h.gate
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Invalidate requests a fresh observation after human input; no mutation is replayed.
func (h *Hub) Invalidate() { h.mu.Lock(); h.next = time.Time{}; h.mu.Unlock() }
