package platform

// hetero (OpenTela-Hetero, not upstream): tests that run on any machine.

import (
	"opentela/internal/common"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Short copy of opentela-hetero resourceprofiler-v1/examples/fixture-kaggle-t4x2.json
const kaggleProfile = `{"schema_version":"1","collected_at":"2026-11-20T20:37:34Z",
"cpu":{"arch":"amd64"},"gpu_status":"ok","gpus":[
{"name":"Tesla T4","memory_total_mib":15360,"memory_used_mib":0,"compute_capability":"7.5"},
{"name":"Tesla T4","memory_total_mib":15360,"memory_used_mib":0,"compute_capability":"7.5"}]}`

func writeFile(t *testing.T, text string) string {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvPicksProfileFile(t *testing.T) {
	t.Setenv(ProfileEnv, writeFile(t, kaggleProfile))
	hw := GetHardwareSpec()
	if hw.CPUArch != "amd64" || hw.GPUStatus != "ok" || hw.CollectedAt != "2026-11-20T20:37:34Z" {
		t.Fatalf("wrong node fields: %+v", hw)
	}
	if len(hw.GPUs) != 2 || hw.GPUs[1].Name != "Tesla T4" ||
		hw.GPUs[1].TotalMemory != 15360 || hw.GPUs[1].ComputeCapability != "7.5" {
		t.Fatalf("wrong gpus: %+v", hw.GPUs)
	}
}

func TestBadProfileFileRefused(t *testing.T) {
	cases := map[string]string{
		"not json":       `{"schema_version":`,
		"wrong version":  `{"schema_version":"2","gpu_status":"none_found","gpus":[]}`,
		"unknown status": `{"schema_version":"1","gpu_status":"maybe","gpus":[]}`,
		"ok but no gpus": `{"schema_version":"1","gpu_status":"ok","gpus":[]}`,
	}
	for name, text := range cases {
		hw := hardwareFromProfileFile(writeFile(t, text))
		if hw.GPUStatus != "query_failed" || hw.CPUArch != "" || len(hw.GPUs) != 0 {
			t.Errorf("%s: should be refused, got %+v", name, hw)
		}
	}
	hw := hardwareFromProfileFile(filepath.Join(t.TempDir(), "missing.json"))
	if hw.GPUStatus != "query_failed" {
		t.Errorf("missing file: should be refused, got %+v", hw)
	}
}

func TestSetComputeCaps(t *testing.T) {
	two := func() []common.GPUSpec { return []common.GPUSpec{{Name: "Tesla T4"}, {Name: "Tesla T4"}} }
	cases := []struct {
		name, out, want string
	}{
		{"names match", "Tesla T4, 7.5\nTesla T4, 7.5", "7.5"},
		{"one line missing", "Tesla T4, 7.5", ""},
		{"other name", "Tesla T4, 7.5\nTesla P100, 6.0", ""},
		{"not a number", "Tesla T4, [N/A]\nTesla T4, [N/A]", ""},
		{"extra comma", "Tesla, T4, 7.5\nTesla T4, 7.5", ""},
	}
	for _, c := range cases {
		gpus := two()
		setComputeCaps(gpus, c.out)
		if gpus[1].ComputeCapability != c.want {
			t.Errorf("%s: got %q, want %q", c.name, gpus[1].ComputeCapability, c.want)
		}
	}
}

func TestLiveReadingOnThisMachine(t *testing.T) {
	t.Setenv(ProfileEnv, "")
	hw := GetHardwareSpec()
	if hw.CPUArch != runtime.GOARCH {
		t.Errorf("cpu_arch %q, want %q", hw.CPUArch, runtime.GOARCH)
	}
	if _, err := time.Parse(time.RFC3339, hw.CollectedAt); err != nil {
		t.Errorf("collected_at %q: %v", hw.CollectedAt, err)
	}
	switch hw.GPUStatus {
	case "ok", "tool_missing", "query_failed":
	default:
		t.Errorf("gpu_status %q", hw.GPUStatus)
	}
	if (hw.GPUStatus == "ok") != (len(hw.GPUs) > 0) {
		t.Errorf("gpu_status %q with %d gpus", hw.GPUStatus, len(hw.GPUs))
	}
	t.Logf("this machine reports: %+v", hw)
}
