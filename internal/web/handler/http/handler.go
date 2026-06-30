package http

import (
	"f1/internal/web/dto"
	
	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	sim Sim
	crossSeason CrossSeason
	data Data
}

func NewHttpHandler(sim Sim, crossSeason CrossSeason, data Data) *HttpHandler {
	return &HttpHandler{
		sim: sim,
		crossSeason: crossSeason,
		data: data,
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
func (h *HttpHandler) GetPilots(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	players, err := h.data.GetPlayersService(ctx)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, players)
}

func (h *HttpHandler) GetTeams(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	teams, err := h.data.GetTeamsService(ctx)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, teams)
}

func (h *HttpHandler) GetPrincipals(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	teamPrincipals, err := h.data.GetPrincipalsService(ctx)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, teamPrincipals)
}

func (h *HttpHandler) GetTrackInfo(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	track := c.Query("track")

	trackInfo, err := h.data.GetTrackInfoService(ctx, track)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, trackInfo)
}

func (h *HttpHandler) GetMyTeam(c *gin.Context) {
	ctx := c.Request.Context() 
	
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	myTeam, err := h.data.GetMyTeamService(ctx, user)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, myTeam)
}

func (h *HttpHandler) GetPlayers(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	players, err := h.data.GetPlayersService(ctx)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, players)
}

func (h *HttpHandler) GetPlayersSquad(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	
	squads, err := h.data.GetPlayersTeamsService(ctx)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, squads)
}