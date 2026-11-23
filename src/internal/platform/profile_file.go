package platform

// hetero (OpenTela-Hetero, not upstream): reads a saved
// resourceprofiler-v1 JSON file, for emulated nodes in experiments.

import (
	"encoding/json"
	"fmt"
	"opentela/internal/common"
	"os"
)

type profileGPU struct {
	Name              string  `json:"name"`
	MemoryTotalMiB    int64   `json:"memory_total_mib"`
	MemoryUsedMiB     int64   `json:"memory_used_mib"`
	ComputeCapability *string `json:"compute_capability"`
}

// profileFile is the part of a resourceprofiler-v1 file that we use.
type profileFile struct {
	SchemaVersion string `json:"schema_version"`
	CollectedAt   string `json:"collected_at"`
	CPU           struct {
		Arch string `json:"arch"`
	} `json:"cpu"`
	GPUStatus string       `json:"gpu_status"`
	GPUs      []profileGPU `json:"gpus"`
}

// hardwareFromProfileFile reports a saved profile. If the file cannot be
// used, the node reports query_failed with no GPUs and no arch, so it
// never pretends to be the real machine.
func hardwareFromProfileFile(path string) common.HardwareSpec {
	hw, err := parseProfile(path)
	if err != nil {
		common.Logger.Error("hetero: cannot use ", ProfileEnv, "=", path, ": ", err)
		return common.HardwareSpec{GPUs: []common.GPUSpec{}, GPUStatus: "query_failed"}
	}
	return hw
}

func parseProfile(path string) (common.HardwareSpec, error) {
	var p profileFile
	data, err := os.ReadFile(path)
	if err != nil {
		return common.HardwareSpec{}, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return common.HardwareSpec{}, err
	}
	if p.SchemaVersion != "1" {
		return common.HardwareSpec{}, fmt.Errorf("schema_version %q, want \"1\"", p.SchemaVersion)
	}
	switch p.GPUStatus {
	case "ok", "none_found", "tool_missing", "query_failed":
	default:
		return common.HardwareSpec{}, fmt.Errorf("unknown gpu_status %q", p.GPUStatus)
	}
	if (p.GPUStatus == "ok") != (len(p.GPUs) > 0) {
		return common.HardwareSpec{}, fmt.Errorf("gpu_status %q with %d gpus", p.GPUStatus, len(p.GPUs))
	}
	hw := common.HardwareSpec{
		GPUs:        []common.GPUSpec{},
		CPUArch:     p.CPU.Arch,
		GPUStatus:   p.GPUStatus,
		CollectedAt: p.CollectedAt,
	}
	for _, g := range p.GPUs {
		spec := common.GPUSpec{Name: g.Name, TotalMemory: g.MemoryTotalMiB, UsedMemory: g.MemoryUsedMiB}
		if g.ComputeCapability != nil {
			spec.ComputeCapability = *g.ComputeCapability
		}
		hw.GPUs = append(hw.GPUs, spec)
	}
	return hw, nil
}
