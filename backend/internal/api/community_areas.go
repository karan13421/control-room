package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"control_room/internal/service"
)

type CommunityAreasHandler struct {
	Service *service.CommunityAreaService
}

func NewCommunityAreasHandler(svc *service.CommunityAreaService) *CommunityAreasHandler {
	return &CommunityAreasHandler{Service: svc}
}

func (h *CommunityAreasHandler) List(c *gin.Context) {
	areas, err := h.Service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list community areas"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"community_areas": areas})
}
