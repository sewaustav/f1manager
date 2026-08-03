package http

import (
	"context"
	"f1/internal/web/dto"
	ws "f1/internal/web/handler/websocket"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ReadyDispatcher собирает готовность игроков перед стартом нового сезона.
type ReadyDispatcher interface {
	Ready(ctx context.Context, groupID, userID int64, totalPlayers int) error

	// CancelGroup drops any pending readiness state for the group — see
	// POST /groups/reset ("end the game early").
	CancelGroup(groupID int64)
}

// PhaseReader — доступ к текущей фазе/этапу группы (in-memory). Clear
// используется POST /groups/reset ("end the game early").
type PhaseReader interface {
	Get(groupID int64) (phase string, stage int64, ok bool)
	Clear(groupID int64)
}

// DraftCanceller — сброс активного драфта группы (POST /groups/reset).
type DraftCanceller interface {
	CancelGroup(groupID int64)
}

// TokenSetupGate копит сабмиты token-setup и открывает первую гонку, когда
// сдала вся группа — без этого фаза token_setup ни во что не переходила.
type TokenSetupGate interface {
	Submitted(groupID, userID int64, totalPlayers int)
	CancelGroup(groupID int64)
}

type HttpHandler struct {
	sim         Sim
	crossSeason CrossSeason
	data        Data
	userData    User
	manager     Manager
	dispatcher  SetupDispatcher
	ready       ReadyDispatcher
	phase       PhaseReader
	draft       DraftCanceller
	tokenSetup  TokenSetupGate
}

func NewHttpHandler(
	sim Sim,
	crossSeason CrossSeason,
	data Data,
	userData User,
	manager Manager,
	dispatcher SetupDispatcher,
	ready ReadyDispatcher,
	phase PhaseReader,
	draft DraftCanceller,
	tokenSetup TokenSetupGate,
) *HttpHandler {
	return &HttpHandler{
		sim:         sim,
		crossSeason: crossSeason,
		data:        data,
		userData:    userData,
		manager:     manager,
		dispatcher:  dispatcher,
		ready:       ready,
		phase:       phase,
		draft:       draft,
		tokenSetup:  tokenSetup,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWs апгрейдит HTTP → WS, регистрирует сессию в менеджере.
// Сессия сама запускает dispatchLoop для входящих сообщений.
// Соединение живёт до тех пор, пока клиент не закроет его.
func (h *HttpHandler) HandleWs(c *gin.Context) {
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

	rawConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	conn := ws.NewConn(rawConn)
	h.manager.Register(user, *groupID, conn)
	// Всё — соединение живёт, горутины внутри conn и session работают сами.
	// Авто-дерегистрация при разрыве происходит в Manager.Register.
}

// --- Sim handlers ---

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

	c.JSON(200, gin.H{"stage": stage, "results": results})
}

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

	c.JSON(200, gin.H{"drivers": drivers, "teams": teams})
}

// --- CrossSeason handlers ---

func (h *HttpHandler) MakeUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	var req dto.Updates
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.MakeUpdate(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

func (h *HttpHandler) MakeSetup(c *gin.Context) {
	ctx := c.Request.Context()

	user, exists := h.getUser(c)
	if !exists {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	var req dto.Setup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.MakeTokenSetup(ctx, user, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Отмечаем сдачу: когда сдаст вся группа, откроется первая гонка и фаза
	// уйдёт из token_setup в racing. Иначе группа зависает здесь навсегда.
	if h.tokenSetup != nil {
		if groupID, err := h.userData.GetUserGroup(ctx, user); err == nil && groupID != nil {
			h.tokenSetup.Submitted(*groupID, user, h.manager.GroupSize(*groupID))
		}
	}

	c.Status(201)
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

// Fire — увольнение пилота или тим-принципала игрока.
func (h *HttpHandler) Fire(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}

	var req dto.Fire
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.crossSeason.Fire(ctx, user, req); err != nil {
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

// --- Data handlers ---

func (h *HttpHandler) GetPilots(c *gin.Context) {
	ctx := c.Request.Context()
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	pilots, err := h.data.GetPilotsService(ctx, user)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pilots)
}

func (h *HttpHandler) GetTeams(c *gin.Context) {
	ctx := c.Request.Context()
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	teams, err := h.data.GetTeamsService(ctx, user)
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
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	players, err := h.data.GetPlayersService(ctx, user)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, players)
}

func (h *HttpHandler) GetPlayersSquad(c *gin.Context) {
	ctx := c.Request.Context()
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	squads, err := h.data.GetPlayersTeamsService(ctx, user)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, squads)
}

func (h *HttpHandler) GetEngines(c *gin.Context) {
	ctx := c.Request.Context()
	if _, exist := h.getUser(c); !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	engines, err := h.data.GetEnginesService(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, engines)
}

func (h *HttpHandler) GetBudget(c *gin.Context) {
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
	budget, tokens, err := h.data.GetBudgetService(ctx, user, *groupID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"budget": budget, "tokens": tokens})
}

// --- Users ---

func (h *HttpHandler) RegisterGroup(c *gin.Context) {
	ctx := c.Request.Context()

	var group dto.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.userData.RegisterGroup(ctx, user, group); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "group registered"})
}

func (h *HttpHandler) JoinGroup(c *gin.Context) {
	ctx := c.Request.Context()

	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "unauthorized"})
		return
	}

	var group dto.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.userData.JoinGroup(ctx, user, group); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "group joined"})
}

// ResetGroup wipes the caller's group back to a fresh pre-draft lobby state —
// "end the game early". Organizer-only: by RegisterGroup's deterministic
// scheme a group's id equals its creator's own userID, so comparing the
// caller against their own group id doubles as the organizer check.
func (h *HttpHandler) ResetGroup(c *gin.Context) {
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
	if user != *groupID {
		c.JSON(403, gin.H{"error": "only the group's organizer can reset it"})
		return
	}

	if err := h.userData.ResetGroup(ctx, *groupID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	h.dispatcher.CancelGroup(*groupID)
	h.draft.CancelGroup(*groupID)
	h.ready.CancelGroup(*groupID)
	h.phase.Clear(*groupID)
	if h.tokenSetup != nil {
		h.tokenSetup.CancelGroup(*groupID)
	}

	c.Status(200)
}

// InitRound — организатор открывает приём сетапов перед этапом.
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

// Ready — игрок подтверждает готовность к старту нового сезона.
// Когда готовы все участники группы, сервер сбрасывает сезон и рассылает
// WS-уведомление season_started.
func (h *HttpHandler) Ready(c *gin.Context) {
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

	total := h.manager.GroupSize(*groupID)
	if err := h.ready.Ready(ctx, *groupID, user, total); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.Status(200)
}

// GetSeasonState — текущая фаза/этап сезона группы игрока и статус текущего раунда.
func (h *HttpHandler) GetSeasonState(c *gin.Context) {
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
	phase, stage, _ := h.phase.Get(*groupID)
	submitted, total, ok := h.dispatcher.RoundState(*groupID)
	if !ok {
		submitted = []int64{}
		total = h.manager.GroupSize(*groupID)
	}
	c.JSON(200, gin.H{
		"phase":            phase,
		"stage":            stage,
		"submitted_setups": submitted,
		"total_players":    total,
	})
}

// --- Трансферные предложения и выход из группы ---

// GetIncomingOffers — предложения выкупить пилота, адресованные игроку.
// Клиент опрашивает этот список: доставка через WS оказалась ненадёжной.
func (h *HttpHandler) GetIncomingOffers(c *gin.Context) {
	ctx := c.Request.Context()
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	offers, err := h.crossSeason.ListIncomingOffers(ctx, user)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, offers)
}

// RespondToOffer — владелец принимает или отклоняет предложение.
func (h *HttpHandler) RespondToOffer(c *gin.Context) {
	ctx := c.Request.Context()
	user, exist := h.getUser(c)
	if !exist {
		c.JSON(403, gin.H{"error": "user not found"})
		return
	}
	var req dto.OfferResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.crossSeason.RespondToOffer(ctx, user, req.OfferID, req.Accept); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(200)
}

// LeaveGroup — выход игрока из группы. Организатор уйти не может: группа
// заводится под его id, без него она осталась бы без владельца — ему нужен
// POST /groups/reset.
func (h *HttpHandler) LeaveGroup(c *gin.Context) {
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
	if user == *groupID {
		c.JSON(400, gin.H{"error": "организатор не может выйти из своей группы — завершите игру"})
		return
	}
	if err := h.userData.LeaveGroup(ctx, user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(200)
}
