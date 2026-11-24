package render

import (
	"encoding/json"
	"io"

	"github.com/rahulkaushal04/netwhy/internal/verdict"
)

// SchemaVersion is the version of the --json output. It increases when a
// field is removed or changes meaning; adding a field does not change it.
const SchemaVersion = 1

type report struct {
	SchemaVersion int    `json:"schema_version"`
	Target        string `json:"target"`
	verdict.Verdict
}

// JSON writes the verdict as indented JSON. Schema version 1:
//
//	schema_version  number   this schema's version
//	target          string   the target as typed
//	ok              bool     true when no layer failed
//	layer           string   the failing layer, absent when ok
//	summary         string   one sentence describing the verdict
//	evidence        object   the failing layer's evidence, empty when ok
//	warnings        array    results with status "warn", absent when none
//	results         array    every layer in the order it ran
//
// A result has name, status ("pass", "warn", "fail" or "skipped"), summary
// and evidence. Evidence has sent, received, duration_ms and error.
func JSON(w io.Writer, target string, v verdict.Verdict) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report{SchemaVersion: SchemaVersion, Target: target, Verdict: v})
}
