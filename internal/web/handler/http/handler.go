package http

import (
	"f1/internal/web/dto"
	
	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	sim Sim
	crossSeason CrossSeason
}

func NewHttpHandler(sim Sim, crossSeason CrossSeason) *HttpHandler {
	return &HttpHandler{
		sim: sim,
		crossSeason: crossSeason,
	}
}

func (h *HttpHandler) MakeSetup(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req dto.Setup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	user, exists := h.getUser(c)
	if !exists {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	if err := h.crossSeason.MakeTokenSetup(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	c.Status(201)
	
}

func (h *HttpHandler) MakeUpdate(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req dto.Updates
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	if err := h.sim.MakeUpdate(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	c.Status(200)

}

func (h *HttpHandler) ChooseSetup(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req dto.RaceSetup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	if err := h.sim.ChooseSetup(ctx, user, req.Setup); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

func (h *HttpHandler) UpdateBase(c *gin.Context) {
	ctx := c.Request.Context()
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	var req dto.BaseUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.UpdateBase(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

func (h *HttpHandler) PickItem(c *gin.Context) {
	ctx := c.Request.Context()
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	var req dto.DraftItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.crossSeason.PickItem(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

// get methods 
