package server

// hetero (OpenTela-Hetero, not upstream): tests for hetero_placement.go.
// No network and no node table needed.

import (
	"net/http"
	"opentela/internal/common"
	"opentela/internal/placement"
	"reflect"
	"testing"
)

func TestHeteroMode(t *testing.T) {
	for in, want := range map[string]string{"": "off", "off": "off", " OFF ": "off", "strict": "strict", "Strict": "strict"} {
		if got, err := heteroMode(in); err != nil || got != want {
			t.Errorf("heteroMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"strct", "lenient", "on"} {
		if _, err := heteroMode(bad); err == nil {
			t.Errorf("heteroMode(%q): want an error", bad)
		}
	}
}

func TestJobFromHeaders(t *testing.T) {
	h := func(kv ...string) http.Header {
		out := http.Header{}
		for i := 0; i < len(kv); i += 2 {
			out.Set(kv[i], kv[i+1])
		}
		return out
	}
	good := []struct {
		name string
		in   http.Header
		want placement.Job
	}{
		{"no headers", h(), placement.Job{}},
		{"all three", h(needArchHeader, "amd64", needMinCCHeader, "8.0", needMinFreeHeader, "10000"), placement.Job{Arch: "amd64", MinCC: "8.0", MinFreeMiB: 10000}},
		{"spaces trimmed", h(needMinCCHeader, " 7.5 "), placement.Job{MinCC: "7.5"}},
		{"0 MiB = no need", h(needMinFreeHeader, "0"), placement.Job{}},
	}
	for _, c := range good {
		if got, err := jobFromHeaders(c.in); err != nil || got != c.want {
			t.Errorf("%s: got %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	for _, bad := range []http.Header{
		h(needMinCCHeader, "eight"), h(needMinCCHeader, "8"),
		h(needMinFreeHeader, "-1"), h(needMinFreeHeader, "lots"), h(needMinFreeHeader, "1.5"),
	} {
		if _, err := jobFromHeaders(bad); err == nil {
			t.Errorf("%v: want an error", bad)
		}
	}
}

// Same readings as the two worker nodes in experiment/modes.md.
var heteroTable = map[string]common.HardwareSpec{
	"rtx": {CPUArch: "amd64", GPUStatus: "ok", GPUs: []common.GPUSpec{{Name: "RTX 5060", TotalMemory: 8151, UsedMemory: 704, ComputeCapability: "12.0"}}},
	"t4":  {CPUArch: "amd64", GPUStatus: "ok", GPUs: []common.GPUSpec{{Name: "T4", TotalMemory: 15360, ComputeCapability: "7.5"}, {Name: "T4", TotalMemory: 15360, ComputeCapability: "7.5"}}},
}

func heteroLookup(id string) (common.HardwareSpec, bool) {
	hw, ok := heteroTable[id]
	return hw, ok
}

func TestFilterByHardware(t *testing.T) {
	all := []string{"t4", "gone", "rtx"} // "gone" is not in the table
	cases := []struct {
		name        string
		job         placement.Job
		wantKept    []string
		wantRefused int
	}{
		{"no needs: untouched, even gone", placement.Job{}, all, 0},
		{"arch amd64: order kept", placement.Job{Arch: "amd64"}, []string{"t4", "rtx"}, 1},
		{"cc 8.0: only rtx", placement.Job{MinCC: "8.0"}, []string{"rtx"}, 2},
		{"10000 MiB free: only t4", placement.Job{MinFreeMiB: 10000}, []string{"t4"}, 2},
		{"both: nobody", placement.Job{MinCC: "8.0", MinFreeMiB: 10000}, nil, 3},
		{"arm64: nobody", placement.Job{Arch: "arm64"}, nil, 3},
	}
	for _, c := range cases {
		kept, refused := filterByHardware(all, c.job, heteroLookup)
		if !reflect.DeepEqual(kept, c.wantKept) || len(refused) != c.wantRefused {
			t.Errorf("%s: kept %v refused %v; want kept %v and %d refused", c.name, kept, refused, c.wantKept, c.wantRefused)
		}
		if c.wantRefused > 0 && refused["gone"] != "not in node table" {
			t.Errorf("%s: gone should be refused as not in node table, got %q", c.name, refused["gone"])
		}
	}
}
