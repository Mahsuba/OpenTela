package platform

// hetero (OpenTela-Hetero, not upstream): fills the capability fields
// added to common.HardwareSpec and common.GPUSpec.

import (
	"context"
	"opentela/internal/common"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ProfileEnv names a resourceprofiler-v1 JSON file. When it is set, the
// node reports that saved profile instead of reading its own hardware.
const ProfileEnv = "OF_HETERO_PROFILE"

var ccPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

// GetHardwareSpec returns this node's hardware with the capability fields filled.
func GetHardwareSpec() common.HardwareSpec {
	if path := os.Getenv(ProfileEnv); path != "" {
		return hardwareFromProfileFile(path)
	}
	return liveHardwareSpec()
}

func liveHardwareSpec() common.HardwareSpec {
	hw := common.HardwareSpec{
		GPUs:        GetGPUInfo(), // ETH's own reading, unchanged
		CPUArch:     runtime.GOARCH,
		CollectedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	switch {
	case len(hw.GPUs) > 0:
		hw.GPUStatus = "ok"
		addComputeCaps(hw.GPUs)
	case !onPath("nvidia-smi") && !onPath("rocm-smi"):
		hw.GPUStatus = "tool_missing"
	default:
		// A GPU tool exists but GetGPUInfo gave nothing. ETH's reader
		// cannot tell "no GPU" from "tool broken", so we say unknown.
		hw.GPUStatus = "query_failed"
	}
	return hw
}

func onPath(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

// addComputeCaps asks nvidia-smi for compute capability (10 s limit).
func addComputeCaps(gpus []common.GPUSpec) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,compute_cap", "--format=csv,noheader").Output()
	if err != nil {
		return
	}
	setComputeCaps(gpus, string(out))
}

// setComputeCaps fills compute capability only if the names line up one to
// one with gpus. Otherwise it leaves the field empty (unknown), never guesses.
func setComputeCaps(gpus []common.GPUSpec, out string) {
	var names, caps []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, ",")
		if len(f) != 2 {
			return
		}
		name := strings.TrimSpace(f[0])
		lower := strings.ToLower(name)
		if name == "" || strings.Contains(lower, "unknown") || strings.Contains(lower, "n/a") {
			continue // same skip rule as getNvidiaGPUs
		}
		names = append(names, name)
		caps = append(caps, strings.TrimSpace(f[1]))
	}
	if len(names) != len(gpus) {
		return
	}
	for i := range gpus {
		if gpus[i].Name != names[i] {
			return
		}
	}
	for i := range gpus {
		if ccPattern.MatchString(caps[i]) {
			gpus[i].ComputeCapability = caps[i]
		}
	}
}
