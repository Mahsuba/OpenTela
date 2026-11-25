package server

// hetero (OpenTela-Hetero, not upstream): reads a request's hardware needs
// and drops providers that cannot meet them. hetero_router.go calls it.

import (
	"fmt"
	"net/http"
	"opentela/internal/common"
	"opentela/internal/placement"
	"regexp"
	"strconv"
	"strings"
)

// A request states its needs with these headers. No header = no need.
const (
	needArchHeader    = "X-Otela-Need-Arch"         // e.g. amd64
	needMinCCHeader   = "X-Otela-Need-Min-CC"       // e.g. 8.0
	needMinFreeHeader = "X-Otela-Need-Min-Free-MiB" // e.g. 10000
)

var ccPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

// heteroMode reads hetero.placement (env OF_HETERO_PLACEMENT).
// "" or "off" = ETH routing, unchanged. "strict" = hardware filter on.
// Any other value is an error, so a typo cannot quietly run the wrong mode.
func heteroMode(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "off":
		return "off", nil
	case "strict":
		return "strict", nil
	}
	return "", fmt.Errorf("hetero.placement must be off or strict, got %q", v)
}

// jobFromHeaders turns the X-Otela-Need-* headers into a placement.Job.
func jobFromHeaders(h http.Header) (placement.Job, error) {
	job := placement.Job{
		Arch:  strings.TrimSpace(h.Get(needArchHeader)),
		MinCC: strings.TrimSpace(h.Get(needMinCCHeader)),
	}
	if job.MinCC != "" && !ccPattern.MatchString(job.MinCC) {
		return placement.Job{}, fmt.Errorf("%s must look like 8.0, got %q", needMinCCHeader, job.MinCC)
	}
	if v := strings.TrimSpace(h.Get(needMinFreeHeader)); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return placement.Job{}, fmt.Errorf("%s must be a whole number of MiB, 0 or more, got %q", needMinFreeHeader, v)
		}
		job.MinFreeMiB = n
	}
	return job, nil
}

// filterByHardware keeps the candidates that can run job, in the same
// order. refused says why each other candidate was dropped. A job with no
// needs returns the candidates untouched, without looking anything up.
func filterByHardware(candidates []string, job placement.Job, hardware func(id string) (common.HardwareSpec, bool)) (kept []string, refused map[string]string) {
	if job == (placement.Job{}) {
		return candidates, nil
	}
	refused = map[string]string{}
	for _, id := range candidates {
		hw, found := hardware(id)
		if !found {
			refused[id] = "not in node table"
			continue
		}
		if ok, why := placement.Compatible(job, hw); ok {
			kept = append(kept, id)
		} else {
			refused[id] = why
		}
	}
	return kept, refused
}
