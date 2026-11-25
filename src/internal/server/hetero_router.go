package server

// hetero (OpenTela-Hetero, not upstream): connects the hardware filter to
// the request path. globalServiceForwardWithScope calls heteroFilter once.

import (
	"net/http"
	"opentela/internal/common"
	"opentela/internal/placement"
	"opentela/internal/protocol"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// heteroFilter returns the candidates to keep. In mode off it returns them
// untouched and never reads the X-Otela-Need-* headers, so routing is
// exactly ETH's. ok is false when it has already written an error reply.
func heteroFilter(c *gin.Context, candidates []string) (kept []string, ok bool) {
	mode, err := heteroMode(viper.GetString("hetero.placement"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	if mode == "off" {
		return candidates, true
	}
	job, err := jobFromHeaders(c.Request.Header)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	kept, refused := filterByHardware(candidates, job, peerHardware)
	if job != (placement.Job{}) {
		common.Logger.Infof("hetero strict: job %+v kept %d of %d, refused %v", job, len(kept), len(candidates), refused)
	}
	if len(kept) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no provider meets the hardware needs", "refused": refused})
		return nil, false
	}
	return kept, true
}

// peerHardware looks a candidate up in this node's copy of the node table.
func peerHardware(id string) (common.HardwareSpec, bool) {
	peer, err := protocol.GetPeerFromTable(id)
	return peer.Hardware, err == nil
}
