// Package probe holds the connectivity checks and the contract they share.
package probe

import (
	"context"
	"net/netip"

	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

type Status string

const (
	Pass Status = "pass"
	// Warn means the check worked but is degraded. It does not block later probes.
	Warn    Status = "warn"
	Fail    Status = "fail"
	Skipped Status = "skipped"
)

func (s Status) Works() bool { return s == Pass || s == Warn }

type Result struct {
	Name     string            `json:"name"`
	Status   Status            `json:"status"`
	Summary  string            `json:"summary"`
	Evidence evidence.Evidence `json:"evidence"`
}

type Probe interface {
	Name() string
	Deps() []string
	// Run must return when ctx is done.
	Run(ctx context.Context) Result
}

// Path passes facts from earlier probes to later ones. Plan.Run finishes a
// probe before starting its dependents, so a probe may read only what one of
// its dependencies writes.
type Path struct {
	Addrs []netip.Addr
	Conn  netip.AddrPort
}
