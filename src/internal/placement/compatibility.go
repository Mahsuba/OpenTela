package placement

// hetero (OpenTela-Hetero, not upstream): answers "can this node run
// this job?" before any node is picked. Not called by the router yet.

import (
	"fmt"
	"opentela/internal/common"
	"strconv"
	"strings"
)

// Job is what a request asks for. An empty field means "no need".
type Job struct {
	Arch       string // CPU architecture, Go name, e.g. "amd64"
	MinCC      string // lowest GPU compute capability, e.g. "8.0"
	MinFreeMiB int64  // free GPU memory needed on one GPU, in MiB
}

// Compatible says whether node can run job. When the answer is no, the
// reason says why. Unknown node values never count as a match.
func Compatible(job Job, node common.HardwareSpec) (bool, string) {
	if job.Arch != "" {
		if node.CPUArch == "" {
			return false, "cpu_arch unknown"
		}
		if node.CPUArch != job.Arch {
			return false, fmt.Sprintf("cpu_arch %s, job wants %s", node.CPUArch, job.Arch)
		}
	}
	if job.MinCC == "" && job.MinFreeMiB <= 0 {
		return true, ""
	}
	if job.MinCC != "" {
		if _, _, ok := parseCC(job.MinCC); !ok {
			return false, fmt.Sprintf("job min_cc %q is not a number", job.MinCC)
		}
	}
	switch node.GPUStatus {
	case "ok":
	case "none_found":
		return false, "no GPU"
	default:
		return false, "gpu_status unknown"
	}
	// The job runs on one GPU, so one GPU must meet every need.
	for _, g := range node.GPUs {
		if gpuFits(job, g) {
			return true, ""
		}
	}
	return false, "no GPU meets min_cc and min_free_mib"
}

func gpuFits(job Job, g common.GPUSpec) bool {
	if job.MinCC != "" {
		wantMajor, wantMinor, _ := parseCC(job.MinCC)
		major, minor, ok := parseCC(g.ComputeCapability)
		if !ok {
			return false // unknown
		}
		if major < wantMajor || (major == wantMajor && minor < wantMinor) {
			return false
		}
	}
	if job.MinFreeMiB > 0 {
		// A broken reading (total 0, or used > total) gives free <= 0,
		// so it never passes. Negative used would make free look bigger
		// than the whole GPU, so it is refused.
		free := g.TotalMemory - g.UsedMemory
		if g.UsedMemory < 0 || free < job.MinFreeMiB {
			return false
		}
	}
	return true
}

// parseCC turns "12.0" into 12 and 0. Comparing the text instead would
// get it wrong: as text, "12.0" comes before "8.0".
func parseCC(s string) (major, minor int, ok bool) {
	a, b, found := strings.Cut(s, ".")
	if !found {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(a)
	minor, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil || major < 0 || minor < 0 {
		return 0, 0, false
	}
	return major, minor, true
}
