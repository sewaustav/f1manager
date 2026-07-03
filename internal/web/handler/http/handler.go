package http

import (
	"f1/internal/web/dto"
	ws "f1/internal/web/handler/websocket"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type HttpHandler struct {
	sim        Sim
	crossSeason CrossSeason
	data        Data
	userData    User
	manager     Manager
	dispatcher  SetupDispatcher
}

func NewHttpHandler(
	sim Sim,
	crossSeason CrossSeason,
	data Data,
	userData User,
	manager Manager,
	dispatcher SetupDispatcher,
) *HttpHandler {
	return &HttpHandler{
		sim:         sim,
		crossSeason: crossSeason,
		data:        data,
		userData:    userData,
		manager:     manager,
		dispatcher:  dispatcher,
	}
}

// --- Sim handlers ---
// Fix service only but handler is fine
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

	if err := h.crossSeason.MakeUpdate(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

// ChooseSetup принимает сетап игрока и передаёт в диспетчер.
// Диспетчер сам следит за готовностью всей группы и запускает симуляцию.
func (h *HttpHandler) ChooseSetup(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.Setup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	groupID, err := h.userData.GetUserGroup(ctx, user)
	if err != nil || groupID == nil {
		c.JSON(400, gin.H{"error": "group not found"})
		return
	}

	if err := h.dispatcher.Submit(ctx, user, *groupID, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

// GetRaceResult возвращает результаты последней гонки группы.
// Done
func (h *HttpHandler) GetRaceResult(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	groupID, err := h.userData.GetUserGroup(ctx, user)
	if err != nil || groupID == nil {
		c.JSON(400, gin.H{"error": "group not found"})
		return
	}

	results, stage, err := h.sim.GetLastRaceResults(ctx, *groupID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"stage":   stage,
		"results": results,
	})
}

// Done
func (h *HttpHandler) GetStanding(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	groupID, err := h.userData.GetUserGroup(ctx, user)
	if err != nil || groupID == nil {
		c.JSON(400, gin.H{"error": "group not found"})
		return
	}

	drivers, teams, err := h.sim.GetStanding(ctx, *groupID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"drivers": drivers,
		"teams":   teams,
	})
}

// --- CrossSeason handlers ---
// TODO - rewrite for  new feature
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


// Handler - fine, Service bad 
// TODO - rewrite service
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

func (h *HttpHandler) PilotTransfer(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	var req dto.PilotTransfer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.PilotTransfer(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

func (h *HttpHandler) PrincipalTransfer(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	var req dto.PrincipalTransfer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.PrincipalTransfer(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

// --- Data handlers ---

func (h *HttpHandler) GetPilots(c *gin.Context) {
	ctx := c.Request.Context()
	_, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	pilots, err := h.data.GetPilotsService(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pilots)
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
		c.JSON(500, gin.H{"error": err.Error()})
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
	principals, err := h.data.GetPrincipalsService(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, principals)
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
		c.JSON(500, gin.H{"error": err.Error()})
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
		c.JSON(500, gin.H{"error": err.Error()})
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
		c.JSON(500, gin.H{"error": err.Error()})
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, squads)
}

// --- WebSocket ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *HttpHandler) HandleWs(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	group, err := h.userData.GetUserGroup(ctx, user)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if group == nil {
		c.JSON(400, gin.H{"error": "user group not found"})
		return
	}

	rawConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	wsConn := ws.NewConn(rawConn)
	session := h.manager.Register(user, *group, *wsConn)

	go func() {
		for {
			select {
			case msg, ok := <-session.Messages():
				if !ok {
					return
				}
				h.handleIncoming(user, msg)
			case <-session.Done():
				return
			}
		}
	}()
}

func (h *HttpHandler) handleIncoming(user int64, msg []byte) {
}

// PickItem — используется в черновике драфта (реализация позже).
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

// InitRound — организатор открывает приём сетапов перед этапом.
// TODO - rewrite can only be used in the start of the sim
func (h *HttpHandler) InitRound(c *gin.Context) {
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	ctx := c.Request.Context()

	groupID, err := h.userData.GetUserGroup(ctx, user)
	if err != nil || groupID == nil {
		c.JSON(400, gin.H{"error": "group not found"})
		return
	}

	stageStr := c.Param("stage")
	stage, err := strconv.ParseInt(stageStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid stage"})
		return
	}

	totalPlayers := h.manager.GroupSize(*groupID)
	h.dispatcher.InitRound(*groupID, stage, totalPlayers)

	c.Status(200)
}