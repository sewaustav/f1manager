package http

import (
	"context"
	"errors"
	"net/http"

	"f1/internal/web/dispatcher"
	"f1/internal/web/dto"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type draftDispatcher interface {
	StartDraft(ctx context.Context, groupID int64) error
	SubmitPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error
}

type draftService interface {
	GetUserGroup(ctx context.Context, userID int64) (*int64, error)
	SwapBotPilots(ctx context.Context, groupID, teamA, teamB, pilotA, pilotB int64) error
}

type DraftHandler struct {
	dispatcher draftDispatcher
	service    draftService
}

func NewDraftHandler(d draftDispatcher, s draftService) *DraftHandler {
	return &DraftHandler{dispatcher: d, service: s}
}

func (h *DraftHandler) RegisterRoutes(rg *gin.RouterGroup, mw *jwtmw.JWTAuthMiddleware) {
	routes := rg.Group("/draft")
	routes.Use(mw.Handler())
	{
		routes.POST("/start", h.Start)
		routes.POST("/pick", h.Pick)
		routes.POST("/bots/swap", h.SwapBots)
	}
}

func (h *DraftHandler) Start(c *gin.Context) {
	userID, ok := draftUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	groupID, err := h.service.GetUserGroup(c.Request.Context(), userID)
	if err != nil || groupID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
		return
	}
	if err := h.dispatcher.StartDraft(c.Request.Context(), *groupID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "draft started"})
}

func (h *DraftHandler) Pick(c *gin.Context) {
	userID, ok := draftUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req dto.Draft
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	groupID, err := h.service.GetUserGroup(c.Request.Context(), userID)
	if err != nil || groupID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
		return
	}

	err = h.dispatcher.SubmitPick(c.Request.Context(), userID, *groupID, req)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	case errors.Is(err, dispatcher.ErrNotYourTurn), errors.Is(err, dispatcher.ErrDraftInactive):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

type swapBotsReq struct {
	TeamA  int64 `json:"team_a"`
	TeamB  int64 `json:"team_b"`
	PilotA int64 `json:"pilot_a"`
	PilotB int64 `json:"pilot_b"`
}

func (h *DraftHandler) SwapBots(c *gin.Context) {
	userID, ok := draftUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req swapBotsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	groupID, err := h.service.GetUserGroup(c.Request.Context(), userID)
	if err != nil || groupID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
		return
	}
	if err := h.service.SwapBotPilots(c.Request.Context(), *groupID, req.TeamA, req.TeamB, req.PilotA, req.PilotB); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func draftUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(jwtmw.UserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok && id > 0
}
