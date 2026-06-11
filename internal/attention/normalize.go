// Package attention is the sensory gate: the normalizer that turns raw
// lines into shapes, the predictor that scores their surprise, and the
// calibrator that judges surprise against each channel's own history.
package attention

import (
	"regexp"
	"strings"
)

// The variable parts of real log lines (timestamps, PIDs, addresses, ports)
// make nearly every raw line unique. A frequency model over raw lines would
// therefore find everything surprising and the thalamus would drop nothing.
// Normalize collapses those variable parts into placeholder tokens so the
// predictor scores the SHAPE of a line. "Ping seq=4731" and "Ping seq=4732"
// are the same event to a prediction engine.
//
// Order matters: structured tokens (UUIDs, timestamps, hex, IPs, paths) are
// collapsed before the generic digit pass so they keep distinct placeholders.
var (
	reUUID = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	reISO  = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b`)
	reSysT = regexp.MustCompile(`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\b`)
	reHexP = regexp.MustCompile(`\b0[xX][0-9a-fA-F]+\b`)
	reHexL = regexp.MustCompile(`\b[0-9a-fA-F]{12,}\b`)
	rePath = regexp.MustCompile(`(?:/[\w.+-]+){2,}/?`)
	reIP   = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?\b`)
	reTime = regexp.MustCompile(`\b\d{1,2}:\d{2}(?::\d{2})?\b`)
	reNum  = regexp.MustCompile(`\d+`)
	reWS   = regexp.MustCompile(`[ \t]+`)
)

// Normalize returns the template of a raw observation. Fragments keep the
// raw content; only prediction runs on the template.
func Normalize(raw string) string {
	s := raw
	s = reUUID.ReplaceAllString(s, "<uuid>")
	s = reISO.ReplaceAllString(s, "<ts>")
	s = reSysT.ReplaceAllString(s, "<ts>")
	s = reHexP.ReplaceAllString(s, "<hex>")
	s = reHexL.ReplaceAllString(s, "<hex>")
	s = rePath.ReplaceAllString(s, "<path>")
	s = reIP.ReplaceAllString(s, "<ip>")
	s = reTime.ReplaceAllString(s, "<ts>")
	s = reNum.ReplaceAllString(s, "<n>")
	s = reWS.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
