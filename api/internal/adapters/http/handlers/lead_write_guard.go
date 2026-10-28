package handlers

import (
	"github.com/gin-gonic/gin"
)

// ensureLeadMutable refuses a write the actor may not make, before the use case
// runs.
//
// These six writes authorised by ACCOUNT only, which produced a pipeline that
// refuses an attendant on every READ and accepts them on every WRITE: they could
// not list or open a lead, and could still change its status, overwrite its draft
// and approve the send. The reads were actor-scoped from the start; the writes
// never were.
//
// `MutableForActor` is the existing rule and the only one this consults —
// gestor and above may change anything in the account, a vendedor may change
// their own leads and unowned ones, and nobody else may change any. A lead
// belonging to somebody else answers the same "forbidden" the pipeline already
// answers, so the write surface stops being wider than the read surface.
//
// Placed at the handler rather than inside each use case on purpose: six use
// cases with six new signatures is six chances to thread the actor differently,
// and the MCP tools call several of them with no actor at all.
func (h *LeadHandler) ensureLeadMutable(c *gin.Context) bool {
	if _, err := h.leadUC.MutableForActor(
		c.Request.Context(), actorFrom(c), c.Param("id")); err != nil {
		respondError(c, err)
		return false
	}
	return true
}
