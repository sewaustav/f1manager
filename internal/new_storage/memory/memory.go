// Package memory — in-memory реализация StaticRepo/DynamicRepo для тестов драфта.
// Draft-методы рабочие; остальное наследуется от stub (возвращает not implemented).
package memory

import (
	"context"
	"errors"
	"sort"
	"sync"

	"f1/internal/models"
	repo "f1/internal/new_storage"
	"f1/internal/new_storage/stub"
)

type Repo struct {
	*stub.Static
	*stub.Dynamic

	mu         sync.Mutex
	players    map[int64]map[int64]*models.Player // group -> userID -> player
	pilots     map[int64]map[int64]*models.Pilot  // group -> pilotID -> pilot
	teams      map[int64]map[int64]*models.Team   // group -> teamID -> team
	cars       map[int64]map[int64]models.Car     // group -> teamID -> car
	principals map[int64]models.TeamPrincipal     // principalID -> principal
	engines    []models.Engine
	offers     map[int64]map[int64]models.TransferOffer // group -> offerID -> offer
	offerSeq   int64
	baseTeams  []models.Team  // шаблоны для GetBaseTeams/сидирования группы
	allPilots  []models.Pilot // общий (не по группам) список — статический GetPilots
}

func New() *Repo {
	return &Repo{
		Static:     &stub.Static{},
		Dynamic:    &stub.Dynamic{},
		players:    map[int64]map[int64]*models.Player{},
		pilots:     map[int64]map[int64]*models.Pilot{},
		teams:      map[int64]map[int64]*models.Team{},
		cars:       map[int64]map[int64]models.Car{},
		principals: map[int64]models.TeamPrincipal{},
	}
}

var (
	_ repo.StaticRepo  = (*Repo)(nil)
	_ repo.DynamicRepo = (*Repo)(nil)
)

// --- seed ---

func (r *Repo) SeedPlayer(groupID int64, p models.Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.players[groupID] == nil {
		r.players[groupID] = map[int64]*models.Player{}
	}
	cp := p
	r.players[groupID][p.ID] = &cp
}

func (r *Repo) SeedPilot(groupID int64, p models.Pilot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pilots[groupID] == nil {
		r.pilots[groupID] = map[int64]*models.Pilot{}
	}
	cp := p
	r.pilots[groupID][p.ID] = &cp
}

func (r *Repo) SeedTeam(groupID int64, t models.Team) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.teams[groupID] == nil {
		r.teams[groupID] = map[int64]*models.Team{}
	}
	cp := t
	r.teams[groupID][t.ID] = &cp
}

func (r *Repo) SeedPrincipal(p models.TeamPrincipal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.principals[p.ID] = p
}

func (r *Repo) SeedEngine(e models.Engine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.engines = append(r.engines, e)
}

// SeedBaseTeams задаёт шаблоны команд, возвращаемые GetBaseTeams (используется
// сервисом при сидировании/сбросе мира группы).
func (r *Repo) SeedBaseTeams(teams []models.Team) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseTeams = append([]models.Team(nil), teams...)
}

func (r *Repo) GetBaseTeams(_ context.Context) ([]models.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Team, len(r.baseTeams))
	copy(out, r.baseTeams)
	return out, nil
}

// SeedStaticPilots задаёт общий (не по группам) список пилотов, возвращаемый
// статическим GetPilots (используется сидированием — не путать с SeedPilot,
// который заполняет пилота ВНУТРИ конкретной группы).
func (r *Repo) SeedStaticPilots(pilots []models.Pilot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allPilots = append([]models.Pilot(nil), pilots...)
}

func (r *Repo) GetPilots(_ context.Context) ([]models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Pilot, len(r.allPilots))
	copy(out, r.allPilots)
	return out, nil
}

// SavePlayer/SaveTeam/SavePilot реализуют DynamicRepo для сидирования —
// поведение идентично Seed*, но идут через интерфейс (используются
// Service.RegisterGroup/ResetGroup напрямую, а не только тестовой подготовкой).
func (r *Repo) SavePlayer(_ context.Context, groupID int64, p models.Player) error {
	r.SeedPlayer(groupID, p)
	return nil
}

func (r *Repo) SaveTeam(_ context.Context, groupID int64, t models.Team) error {
	r.SeedTeam(groupID, t)
	return nil
}

func (r *Repo) SavePilot(_ context.Context, groupID int64, p models.Pilot) error {
	r.SeedPilot(groupID, p)
	return nil
}

// --- dynamic reads ---

func (r *Repo) UpdatePlayer(_ context.Context, _, groupID int64, p models.Player) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.players[groupID] == nil {
		r.players[groupID] = map[int64]*models.Player{}
	}
	cp := p
	r.players[groupID][p.ID] = &cp
	return nil
}

func (r *Repo) GetPlayer(_ context.Context, userID, groupID int64) (models.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.players[groupID]; ok {
		if p, ok := g[userID]; ok {
			return *p, nil
		}
	}
	return models.Player{}, errors.New("player not found")
}

func (r *Repo) GetPlayers(_ context.Context, groupID int64) ([]models.Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Player
	for _, p := range r.players[groupID] {
		out = append(out, *p)
	}
	return out, nil
}

func (r *Repo) GetBudget(_ context.Context, userID, groupID int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.players[groupID]; ok {
		if p, ok := g[userID]; ok {
			return p.Budget, nil
		}
	}
	return 0, errors.New("player not found")
}

func (r *Repo) GetTeamByGroup(_ context.Context, teamID, groupID int64) (models.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.teams[groupID]; ok {
		if t, ok := g[teamID]; ok {
			return *t, nil
		}
	}
	return models.Team{}, errors.New("team not found")
}

func (r *Repo) GetTeamsByGroup(_ context.Context, groupID int64) ([]models.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Team
	for _, t := range r.teams[groupID] {
		out = append(out, *t)
	}
	return out, nil
}

func (r *Repo) GetPilotsByGroup(_ context.Context, groupID int64) ([]models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Pilot
	for _, p := range r.pilots[groupID] {
		out = append(out, *p)
	}
	return out, nil
}

func (r *Repo) GetPilotByGroup(_ context.Context, pilotID, groupID int64) (models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.pilots[groupID]; ok {
		if p, ok := g[pilotID]; ok {
			return *p, nil
		}
	}
	return models.Pilot{}, errors.New("pilot not found")
}

func (r *Repo) GetPlayerPilots(_ context.Context, userID, groupID int64) ([]models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Pilot
	for _, p := range r.pilots[groupID] {
		if p.Team != nil && *p.Team == userID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *Repo) GetUnassignedPilots(_ context.Context, groupID int64) ([]models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Pilot
	for _, p := range r.pilots[groupID] {
		// Не распределён = нет владельца-игрока и нет гаража (0/nil).
		if p.Team == nil && (p.Garage == nil || *p.Garage == 0) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *Repo) GetPilotsByTeam(_ context.Context, teamID, groupID int64) ([]models.Pilot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Pilot
	for _, p := range r.pilots[groupID] {
		if p.Garage != nil && *p.Garage == teamID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *Repo) GetBotTeams(_ context.Context, groupID int64) ([]models.Team, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	owned := map[int64]bool{}
	for _, p := range r.players[groupID] {
		if p.Team != 0 {
			owned[p.Team] = true
		}
	}
	var out []models.Team
	for _, t := range r.teams[groupID] {
		if !owned[t.ID] {
			out = append(out, *t)
		}
	}
	return out, nil
}

// --- dynamic writes ---

func (r *Repo) SetPlayerTeam(_ context.Context, userID, groupID, teamID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.player(groupID, userID)
	if p == nil {
		return errors.New("player not found")
	}
	p.Team = teamID
	return nil
}

func (r *Repo) SetPlayerBudget(_ context.Context, userID, groupID int64, budget int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.player(groupID, userID)
	if p == nil {
		return errors.New("player not found")
	}
	p.Budget = budget
	return nil
}

func (r *Repo) SetPlayerPrincipal(_ context.Context, userID, groupID, principalID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.player(groupID, userID)
	if p == nil {
		return errors.New("player not found")
	}
	id := principalID
	p.TeamPrincipal = &id
	return nil
}

func (r *Repo) SetPilotOwner(_ context.Context, pilotID, groupID int64, owner *int64, garage *int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g := r.pilots[groupID]
	if g == nil || g[pilotID] == nil {
		return errors.New("pilot not found")
	}
	p := g[pilotID]
	if owner != nil {
		v := *owner
		p.Team = &v
	} else {
		p.Team = nil
	}
	if garage != nil {
		v := *garage
		p.Garage = &v
	} else {
		p.Garage = nil
	}
	return nil
}

func (r *Repo) SetTeamEngine(_ context.Context, teamID, groupID int64, ice models.ICEName) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g := r.teams[groupID]
	if g == nil || g[teamID] == nil {
		return errors.New("team not found")
	}
	g[teamID].ICE = ice
	return nil
}

// --- static reads (draft-relevant) ---

func (r *Repo) GetTeamPrincipal(_ context.Context, principalID int64) (models.TeamPrincipal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.principals[principalID]; ok {
		return p, nil
	}
	return models.TeamPrincipal{}, errors.New("principal not found")
}

func (r *Repo) GetEngines(_ context.Context) ([]models.Engine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Engine, len(r.engines))
	copy(out, r.engines)
	return out, nil
}

// --- tokens / cars / group (для ChooseSetup и смежной логики) ---

func (r *Repo) GetTokens(_ context.Context, userID, groupID int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.player(groupID, userID)
	if p == nil {
		return 0, errors.New("player not found")
	}
	return p.Tokens, nil
}

func (r *Repo) UpdateTokens(_ context.Context, userID, groupID int64, tokens int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.player(groupID, userID)
	if p == nil {
		return errors.New("player not found")
	}
	p.Tokens = tokens
	return nil
}

func (r *Repo) UpdateCar(_ context.Context, teamID, groupID int64, car models.Car) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cars[groupID] == nil {
		r.cars[groupID] = map[int64]models.Car{}
	}
	r.cars[groupID][teamID] = car
	return nil
}

func (r *Repo) GetCar(_ context.Context, teamID, groupID int64) (models.Car, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.cars[groupID]; ok {
		if c, ok := g[teamID]; ok {
			return c, nil
		}
	}
	return models.Car{}, errors.New("car not found")
}

func (r *Repo) GetUserGroup(_ context.Context, userID int64) (*int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for groupID, g := range r.players {
		if _, ok := g[userID]; ok {
			id := groupID
			return &id, nil
		}
	}
	return nil, nil
}

func (r *Repo) player(groupID, userID int64) *models.Player {
	if g, ok := r.players[groupID]; ok {
		return g[userID]
	}
	return nil
}

// --- трансферные предложения ---

func (r *Repo) CreateTransferOffer(_ context.Context, groupID int64, o models.TransferOffer) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.offers == nil {
		r.offers = map[int64]map[int64]models.TransferOffer{}
	}
	if r.offers[groupID] == nil {
		r.offers[groupID] = map[int64]models.TransferOffer{}
	}
	r.offerSeq++
	o.ID = r.offerSeq
	r.offers[groupID][o.ID] = o
	return o.ID, nil
}

func (r *Repo) GetTransferOffer(_ context.Context, groupID, offerID int64) (models.TransferOffer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.offers[groupID]; ok {
		if o, ok := g[offerID]; ok {
			return o, nil
		}
	}
	return models.TransferOffer{}, errors.New("offer not found")
}

func (r *Repo) ListTransferOffers(_ context.Context, groupID int64) ([]models.TransferOffer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.TransferOffer, 0, len(r.offers[groupID]))
	for _, o := range r.offers[groupID] {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *Repo) DeleteTransferOffer(_ context.Context, groupID, offerID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.offers[groupID]; ok {
		delete(g, offerID)
	}
	return nil
}

func (r *Repo) LeaveGroup(ctx context.Context, userID, groupID int64) error {
	pilots, err := r.GetPlayerPilots(ctx, userID, groupID)
	if err != nil {
		return err
	}
	for _, p := range pilots {
		if err := r.SetPilotOwner(ctx, p.ID, groupID, nil, nil); err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.players[groupID]; ok {
		delete(g, userID)
	}
	return nil
}

func (r *Repo) UpdateBudget(_ context.Context, userID, groupID int64, delta int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.players[groupID]
	if !ok {
		return errors.New("group not found")
	}
	p, ok := g[userID]
	if !ok {
		return errors.New("player not found")
	}
	p.Budget += delta
	return nil
}

// ExecutePilotTransfer переносит пилота новому владельцу и в его гараж —
// как и в redis-реализации, иначе пилот не попадает в состав команды.
func (r *Repo) ExecutePilotTransfer(_ context.Context, pilotID, _, toTeamID int64, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var groupID int64 = -1
	for g, players := range r.players {
		if _, ok := players[toTeamID]; ok {
			groupID = g
			break
		}
	}
	if groupID < 0 {
		return errors.New("buyer not in any group")
	}
	p, ok := r.pilots[groupID][pilotID]
	if !ok {
		return errors.New("pilot not found")
	}
	owner := toTeamID
	p.Team = &owner
	if buyer, ok := r.players[groupID][toTeamID]; ok && buyer.Team != 0 {
		garage := buyer.Team
		p.Garage = &garage
	}
	return nil
}

func (r *Repo) ClearGroup(_ context.Context, groupID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.players, groupID)
	delete(r.offers, groupID)
	return nil
}

func (r *Repo) RegisterGroup(ctx context.Context, userID int64, _, _ string) error {
	// groupID == id организатора, как и в redis-реализации.
	return r.SavePlayer(ctx, userID, models.Player{ID: userID})
}
