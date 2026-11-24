package placement

// hetero (OpenTela-Hetero, not upstream): compatible and incompatible cases.

import (
	"opentela/internal/common"
	"testing"
)

func node(arch, status string, gpus ...common.GPUSpec) common.HardwareSpec {
	return common.HardwareSpec{CPUArch: arch, GPUStatus: status, GPUs: gpus}
}

func gpu(cc string, total, used int64) common.GPUSpec {
	return common.GPUSpec{Name: "GPU", TotalMemory: total, UsedMemory: used, ComputeCapability: cc}
}

var (
	// Real readings: Mahi's PC (W6D3 log) and the Kaggle capture (2026-11-20).
	rtx5060 = node("amd64", "ok", gpu("12.0", 8151, 704))
	kaggle  = node("amd64", "ok", gpu("7.5", 15360, 0), gpu("7.5", 15360, 0))

	// Hand-written nodes (not captures).
	oldNode  = node("", "", gpu("", 8000, 0)) // sent by a node without our fields
	noGPU    = node("arm64", "none_found")
	noTool   = node("amd64", "tool_missing")
	badMem   = node("amd64", "ok", gpu("8.0", 100, 900))
	zeroMem  = node("amd64", "ok", gpu("8.0", 0, 0))
	negUsed  = node("amd64", "ok", gpu("8.0", 8000, -500))
	noCC     = node("amd64", "ok", gpu("", 16384, 0))
	splitGPU = node("amd64", "ok", gpu("8.6", 8000, 7000), gpu("7.5", 16000, 0))
)

func TestCompatible(t *testing.T) {
	cases := []struct {
		name string
		job  Job
		node common.HardwareSpec
		want bool
	}{
		{"no needs, real node", Job{}, rtx5060, true},
		{"no needs, old node", Job{}, oldNode, true},
		{"no needs, no GPU", Job{}, noGPU, true},
		{"arch matches", Job{Arch: "amd64"}, rtx5060, true},
		{"arch differs", Job{Arch: "arm64"}, rtx5060, false},
		{"arch unknown", Job{Arch: "amd64"}, oldNode, false},
		{"arch only, no GPU", Job{Arch: "arm64"}, noGPU, true},
		{"cc 12.0 >= 8.0 (as text it would fail)", Job{MinCC: "8.0"}, rtx5060, true},
		{"cc 7.5 < 8.0", Job{MinCC: "8.0"}, kaggle, false},
		{"cc equal", Job{MinCC: "7.5"}, kaggle, true},
		{"cc unknown", Job{MinCC: "8.0"}, noCC, false},
		{"job cc not a number", Job{MinCC: "eight"}, rtx5060, false},
		{"free 7447 < 10000", Job{MinFreeMiB: 10000}, rtx5060, false},
		{"free 15360 >= 10000", Job{MinFreeMiB: 10000}, kaggle, true},
		{"free only, cc unknown", Job{MinFreeMiB: 10000}, noCC, true},
		{"used > total", Job{MinFreeMiB: 1}, badMem, false},
		{"total 0", Job{MinFreeMiB: 1}, zeroMem, false},
		{"used below 0", Job{MinFreeMiB: 8200}, negUsed, false},
		{"GPU job, no GPU", Job{MinFreeMiB: 1}, noGPU, false},
		{"GPU job, tool missing", Job{MinFreeMiB: 1}, noTool, false},
		{"GPU job, old node", Job{MinFreeMiB: 1}, oldNode, false},
		{"each need met by a different GPU", Job{MinCC: "8.0", MinFreeMiB: 4000}, splitGPU, false},
		{"one GPU meets both", Job{MinCC: "7.0", MinFreeMiB: 4000}, splitGPU, true},
	}
	for _, c := range cases {
		got, reason := Compatible(c.job, c.node)
		if got != c.want {
			t.Errorf("%s: got %v (%s), want %v", c.name, got, reason, c.want)
		}
		if !got && reason == "" {
			t.Errorf("%s: no reason given", c.name)
		}
	}
}
