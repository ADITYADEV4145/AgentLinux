// SPDX-License-Identifier: Apache-2.0
// Package cloud is the development control plane and remote machine adapter.
// No provider resources are created unless an operator separately enables one.
package cloud

import (
	"context"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"time"
)

const Version = "cloud-prototype-1"

type Limits struct {
	MaxSessions   int   `json:"maxSessions"`
	MaxPerOwner   int   `json:"maxPerOwner"`
	MaxSeconds    int   `json:"maxSeconds"`
	IdleSeconds   int   `json:"idleSeconds"`
	BudgetSeconds int64 `json:"budgetSeconds"`
}

func DefaultLimits() Limits { return Limits{2, 1, 900, 300, 1800} }
func (l Limits) valid() bool {
	return l.MaxSessions > 0 && l.MaxSessions <= 4 && l.MaxPerOwner > 0 && l.MaxPerOwner <= l.MaxSessions && l.MaxSeconds >= 30 && l.MaxSeconds <= 900 && l.IdleSeconds >= 10 && l.IdleSeconds <= l.MaxSeconds && l.BudgetSeconds >= int64(l.MaxSeconds) && l.BudgetSeconds <= 14400
}

type Identity struct {
	ID        string    `json:"id"`
	Workspace string    `json:"workspace"`
	TokenHash string    `json:"tokenHash"`
	Expires   time.Time `json:"expires"`
	Revoked   bool      `json:"revoked"`
}
type Session struct {
	EstimatedSeconds int64                  `json:"estimatedRuntimeSeconds"`
	IdleStopAt       time.Time              `json:"idleStopAt"`
	Policy           *Limits                `json:"limits,omitempty"`
	ID               string                 `json:"id"`
	Owner            string                 `json:"owner"`
	Workspace        string                 `json:"workspace"`
	Key              string                 `json:"-"`
	RequestKey       string                 `json:"requestKey"`
	State            string                 `json:"state"`
	ProviderID       string                 `json:"providerId,omitempty"`
	Provider         string                 `json:"provider"`
	Simulated        bool                   `json:"simulated"`
	Attempted        bool                   `json:"attempted"`
	Created          time.Time              `json:"created"`
	Expires          time.Time              `json:"expires"`
	LastAction       time.Time              `json:"lastAction"`
	StoppedAt        time.Time              `json:"stoppedAt,omitempty"`
	ObservedSeconds  int64                  `json:"observedSeconds"`
	Error            string                 `json:"error,omitempty"`
	Generation       string                 `json:"generation,omitempty"`
	Capabilities     *protocol.Capabilities `json:"capabilities,omitempty"`
}
type Database struct {
	Version    string              `json:"version"`
	Deployment string              `json:"deployment"`
	Provider   string              `json:"provider"`
	Limits     Limits              `json:"limits"`
	Identities map[string]Identity `json:"identities"`
	Sessions   map[string]Session  `json:"sessions"`
}
type Create struct {
	Workspace string `json:"workspace"`
	Seconds   int    `json:"seconds"`
	Profile   string `json:"profile"`
}

// Resources and operation results are provider observations, never process-start readiness.
// Create must label resources by deployment/session before returning. List must
// enumerate all owned live resources or fail, never silently truncate inventory.
type Resource struct {
	ID         string `json:"id"`
	Session    string `json:"session"`
	Deployment string `json:"deployment"`
	State      string `json:"state"`
}
type Probe struct {
	Ready        bool                   `json:"ready"`
	Generation   string                 `json:"generation"`
	Capabilities *protocol.Capabilities `json:"capabilities"`
}
type Operation struct {
	ControlNonce string                `json:"controlNonce,omitempty"`
	Method       string                `json:"method"`
	Input        *protocol.Input       `json:"input,omitempty"`
	Exec         *protocol.ExecRequest `json:"exec,omitempty"`
	App          string                `json:"app,omitempty"`
}
type Output struct {
	Frame      *protocol.Frame       `json:"frame,omitempty"`
	PNG        []byte                `json:"png,omitempty"`
	Generation string                `json:"generation,omitempty"`
	Input      *protocol.InputResult `json:"input,omitempty"`
	Execution  *protocol.Execution   `json:"execution,omitempty"`
}
type Provider interface {
	Name() string
	Simulated() bool
	Create(context.Context, string, Session) (Resource, error)
	List(context.Context, string) ([]Resource, error)
	Probe(context.Context, Session) (Probe, error)
	Call(context.Context, Session, Operation) (Output, error)
	Delete(context.Context, Resource) error // Idempotent; return nil only if absent/terminated.
}
