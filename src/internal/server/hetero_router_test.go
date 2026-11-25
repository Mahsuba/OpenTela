package server

// hetero (OpenTela-Hetero, not upstream): tests for hetero_router.go with
// ETH's real handler and node table. Uses heteroTable from
// hetero_placement_test.go.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"opentela/internal/protocol"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	ds "github.com/ipfs/go-datastore"
	"github.com/spf13/viper"
)

// heteroSetup puts the rtx and t4 readings into the node table as
// providers of service "echo", and sets the mode.
func heteroSetup(t *testing.T, mode string) {
	t.Helper()
	resetNodeTableForTest()
	viper.Set("security.require_signed_binary", false)
	viper.Set("hetero.placement", mode)
	t.Cleanup(func() {
		viper.Set("hetero.placement", "")
		resetNodeTableForTest()
	})
	for id, hw := range heteroTable {
		peer := protocol.Peer{ID: id, Connected: true, Hardware: hw,
			Service: []protocol.Service{{Name: "echo", IdentityGroup: []string{"all"}}}}
		body, err := json.Marshal(peer)
		if err != nil {
			t.Fatal(err)
		}
		protocol.UpdateNodeTableHook(ds.NewKey(id), body)
	}
}

// heteroRequest sends one request through ETH's real handler.
func heteroRequest(needs ...string) *httptest.ResponseRecorder {
	r := gin.New()
	r.POST("/v1/service/:service/*path", GlobalServiceForwardHandler)
	req, _ := http.NewRequest(http.MethodPost, "/v1/service/echo/test", bytes.NewBufferString(`{}`))
	req.Header.Set("X-Otela-Fallback", "2")
	for i := 0; i+1 < len(needs); i += 2 {
		req.Header.Set(needs[i], needs[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHeteroStrictNobodyFits(t *testing.T) {
	heteroSetup(t, "strict")
	w := heteroRequest(needMinCCHeader, "8.0", needMinFreeHeader, "10000")
	body := w.Body.String()
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(body, "no provider meets the hardware needs") ||
		!strings.Contains(body, `"rtx":`) || !strings.Contains(body, `"t4":`) {
		t.Fatalf("got %d %s", w.Code, body)
	}
}

func TestHeteroBadInput(t *testing.T) {
	heteroSetup(t, "strict")
	if w := heteroRequest(needMinCCHeader, "eight"); w.Code != http.StatusBadRequest {
		t.Errorf("bad min_cc: got %d %s", w.Code, w.Body.String())
	}
	viper.Set("hetero.placement", "strct")
	if w := heteroRequest(); w.Code != http.StatusInternalServerError {
		t.Errorf("mode typo: got %d %s", w.Code, w.Body.String())
	}
}

// heteroFilterOnly runs heteroFilter alone, so nothing is forwarded.
func heteroFilterOnly(t *testing.T, candidates []string, needs ...string) ([]string, bool, string) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	for i := 0; i+1 < len(needs); i += 2 {
		c.Request.Header.Set(needs[i], needs[i+1])
	}
	kept, ok := heteroFilter(c, candidates)
	return kept, ok, w.Body.String()
}

func TestHeteroStrictKeepsOnlyFit(t *testing.T) {
	heteroSetup(t, "strict")
	kept, ok, body := heteroFilterOnly(t, []string{"t4", "rtx"}, needMinCCHeader, "8.0")
	if !ok || !reflect.DeepEqual(kept, []string{"rtx"}) || body != "" {
		t.Fatalf("got %v %v %q", kept, ok, body)
	}
}

func TestHeteroOffIgnoresNeeds(t *testing.T) {
	heteroSetup(t, "off")
	in := []string{"t4", "rtx"}
	kept, ok, body := heteroFilterOnly(t, in, needMinCCHeader, "eight", needArchHeader, "arm64")
	if !ok || !reflect.DeepEqual(kept, in) || body != "" {
		t.Fatalf("got %v %v %q", kept, ok, body)
	}
}
