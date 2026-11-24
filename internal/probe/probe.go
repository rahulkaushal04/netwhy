package probe

import (
	"context"
	"github.com/rahulkaushal04/netwhy/internal/evidence"
)

type Status string

const (
	Pass    Status = "pass"
	Fail    Status = "fail"
	Skipped Status = "skipped"
)

type Result struct {
	Name     string            `json:"name"`
	Status   Status            `json:"status"`
	Summary  string            `json:"summary"`
	Evidence evidence.Evidence `json:"evidence"`
}

type Probe interface {
	Name() string
	Deps() []string
	Run(ctx context.Context) Result
}
