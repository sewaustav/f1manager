// Package redis — Redis-реализация DynamicRepo с групповой изоляцией.
// Каждая сущность сериализуется в JSON и хранится под ключом с префиксом
// g:{groupID}:..., что обеспечивает изоляцию состояния разных групп.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"f1/internal/models"
	repo "f1/internal/new_storage"

	goredis "github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("redis: not found")

type Dynamic struct {
	rdb *goredis.Client
}

func NewDynamic(rdb *goredis.Client) *Dynamic {
	return &Dynamic{rdb: rdb}
}

var _ repo.DynamicRepo = (*Dynamic)(nil)

// --- ключи ---

func playerKey(g, uid int64) string  { return fmt.Sprintf("g:%d:player:%d", g, uid) }
func playersIdx(g int64) string      { return fmt.Sprintf("g:%d:players", g) }
func teamKey(g, tid int64) string    { return fmt.Sprintf("g:%d:team:%d", g, tid) }
func teamsIdx(g int64) string        { return fmt.Sprintf("g:%d:teams", g) }
func pilotKey(g, pid int64) string   { return fmt.Sprintf("g:%d:pilot:%d", g, pid) }
func pilotsIdx(g int64) string       { return fmt.Sprintf("g:%d:pilots", g) }
func carKey(g, tid int64) string     { return fmt.Sprintf("g:%d:car:%d", g, tid) }
func ptKey(g, pid, tid int64) string { return fmt.Sprintf("g:%d:pt:%d:%d", g, pid, tid) }
func standDriversKey(g int64) string { return fmt.Sprintf("g:%d:standing:drivers", g) }
func standTeamsKey(g int64) string   { return fmt.Sprintf("g:%d:standing:teams", g) }
func lastRaceKey(g int64) string     { return fmt.Sprintf("g:%d:lastrace", g) }
func lastStageKey(g int64) string    { return fmt.Sprintf("g:%d:laststage", g) }
func groupMetaKey(g int64) string    { return fmt.Sprintf("g:%d:meta", g) }
func groupByNameKey(n string) string { return fmt.Sprintf("groups:name:%s", n) }
func userGroupKey(uid int64) string  { return fmt.Sprintf("user:%d:group", uid) }

// --- JSON-хелперы ---

func (d *Dynamic) setJSON(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return d.rdb.Set(ctx, key, b, 0).Err()
}

func (d *Dynamic) getJSON(ctx context.Context, key string, v any) (bool, error) {
	s, err := d.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(s), v)
}

func (d *Dynamic) members(ctx context.Context, idx string) ([]int64, error) {
	vals, err := d.rdb.SMembers(ctx, idx).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(vals))
	for _, v := range vals {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- сохранение (используется сидированием и внутренними операциями) ---

func (d *Dynamic) SavePlayer(ctx context.Context, groupID int64, p models.Player) error {
	if err := d.setJSON(ctx, playerKey(groupID, p.ID), p); err != nil {
		return err
	}
	if err := d.rdb.SAdd(ctx, playersIdx(groupID), p.ID).Err(); err != nil {
		return err
	}
	return d.rdb.Set(ctx, userGroupKey(p.ID), groupID, 0).Err()
}

func (d *Dynamic) SaveTeam(ctx context.Context, groupID int64, t models.Team) error {
	if err := d.setJSON(ctx, teamKey(groupID, t.ID), t); err != nil {
		return err
	}
	return d.rdb.SAdd(ctx, teamsIdx(groupID), t.ID).Err()
}

func (d *Dynamic) SavePilot(ctx context.Context, groupID int64, p models.Pilot) error {
	if err := d.setJSON(ctx, pilotKey(groupID, p.ID), p); err != nil {
		return err
	}
	return d.rdb.SAdd(ctx, pilotsIdx(groupID), p.ID).Err()
}

// --- reads ---

func (d *Dynamic) GetPlayer(ctx context.Context, userID, groupID int64) (models.Player, error) {
	var p models.Player
	ok, err := d.getJSON(ctx, playerKey(groupID, userID), &p)
	if err != nil {
		return models.Player{}, err
	}
	if !ok {
		return models.Player{}, ErrNotFound
	}
	return p, nil
}

func (d *Dynamic) GetPlayers(ctx context.Context, groupID int64) ([]models.Player, error) {
	ids, err := d.members(ctx, playersIdx(groupID))
	if err != nil {
		return nil, err
	}
	out := make([]models.Player, 0, len(ids))
	for _, id := range ids {
		var p models.Player
		ok, err := d.getJSON(ctx, playerKey(groupID, id), &p)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Dynamic) GetPilotsByGroup(ctx context.Context, groupID int64) ([]models.Pilot, error) {
	ids, err := d.members(ctx, pilotsIdx(groupID))
	if err != nil {
		return nil, err
	}
	out := make([]models.Pilot, 0, len(ids))
	for _, id := range ids {
		var p models.Pilot
		ok, err := d.getJSON(ctx, pilotKey(groupID, id), &p)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Dynamic) GetTeamsByGroup(ctx context.Context, groupID int64) ([]models.Team, error) {
	ids, err := d.members(ctx, teamsIdx(groupID))
	if err != nil {
		return nil, err
	}
	out := make([]models.Team, 0, len(ids))
	for _, id := range ids {
		var t models.Team
		ok, err := d.getJSON(ctx, teamKey(groupID, id), &t)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (d *Dynamic) GetTeamByGroup(ctx context.Context, teamID, groupID int64) (models.Team, error) {
	var t models.Team
	ok, err := d.getJSON(ctx, teamKey(groupID, teamID), &t)
	if err != nil {
		return models.Team{}, err
	}
	if !ok {
		return models.Team{}, ErrNotFound
	}
	return t, nil
}

func (d *Dynamic) GetCar(ctx context.Context, teamID, groupID int64) (models.Car, error) {
	var c models.Car
	ok, err := d.getJSON(ctx, carKey(groupID, teamID), &c)
	if err != nil {
		return models.Car{}, err
	}
	if !ok {
		return models.Car{}, ErrNotFound
	}
	return c, nil
}

func (d *Dynamic) GetBudget(ctx context.Context, userID, groupID int64) (int, error) {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return 0, err
	}
	return p.Budget, nil
}

func (d *Dynamic) GetTokens(ctx context.Context, userID, groupID int64) (int, error) {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return 0, err
	}
	return p.Tokens, nil
}

func (d *Dynamic) GetStanding(ctx context.Context, groupID int64) (map[int64]int, map[int64]int, error) {
	drivers, err := d.readPoints(ctx, standDriversKey(groupID))
	if err != nil {
		return nil, nil, err
	}
	teams, err := d.readPoints(ctx, standTeamsKey(groupID))
	if err != nil {
		return nil, nil, err
	}
	return drivers, teams, nil
}

func (d *Dynamic) readPoints(ctx context.Context, key string) (map[int64]int, error) {
	m, err := d.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(m))
	for k, v := range m {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		pts, _ := strconv.Atoi(v)
		out[id] = pts
	}
	return out, nil
}

func (d *Dynamic) GetLastRaceResults(ctx context.Context, groupID int64) ([]models.RaceResult, int64, error) {
	var results []models.RaceResult
	if _, err := d.getJSON(ctx, lastRaceKey(groupID), &results); err != nil {
		return nil, 0, err
	}
	stage, err := d.rdb.Get(ctx, lastStageKey(groupID)).Int64()
	if errors.Is(err, goredis.Nil) {
		return results, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return results, stage, nil
}

// --- race ---

func (d *Dynamic) HandleRace(ctx context.Context, race []models.RaceResult, groupID int64) error {
	if err := d.setJSON(ctx, lastRaceKey(groupID), race); err != nil {
		return err
	}
	for _, r := range race {
		if err := d.rdb.HIncrBy(ctx, standDriversKey(groupID), strconv.FormatInt(r.PilotID, 10), int64(r.Points)).Err(); err != nil {
			return err
		}
		if r.GarageID != 0 {
			if err := d.rdb.HIncrBy(ctx, standTeamsKey(groupID), strconv.FormatInt(r.GarageID, 10), int64(r.Points)).Err(); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- updates ---

func (d *Dynamic) UpdateCar(ctx context.Context, teamID, groupID int64, car models.Car) error {
	return d.setJSON(ctx, carKey(groupID, teamID), car)
}

func (d *Dynamic) UpdateTeam(ctx context.Context, userID int64, team models.Team) error {
	groupID, err := d.resolveGroup(ctx, userID)
	if err != nil {
		return err
	}
	return d.setJSON(ctx, teamKey(groupID, team.ID), team)
}

func (d *Dynamic) UpdatePlayer(ctx context.Context, userID, groupID int64, player models.Player) error {
	return d.SavePlayer(ctx, groupID, player)
}

func (d *Dynamic) UpdateBudget(ctx context.Context, userID, groupID int64, delta int) error {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}
	p.Budget += delta
	return d.setJSON(ctx, playerKey(groupID, userID), p)
}

func (d *Dynamic) UpdateTokens(ctx context.Context, userID, groupID int64, tokens int) error {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}
	p.Tokens = tokens
	return d.setJSON(ctx, playerKey(groupID, userID), p)
}

// --- transfers ---

func (d *Dynamic) ExecutePilotTransfer(ctx context.Context, pilotID, fromTeamID, toTeamID int64, cost int) error {
	// toTeamID — новый владелец-игрок; через него выводим группу.
	groupID, err := d.resolveGroup(ctx, toTeamID)
	if err != nil {
		return err
	}
	var pilot models.Pilot
	ok, err := d.getJSON(ctx, pilotKey(groupID, pilotID), &pilot)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	owner := toTeamID
	pilot.Team = &owner
	return d.setJSON(ctx, pilotKey(groupID, pilotID), pilot)
}

func (d *Dynamic) ExecutePrincipalTransfer(ctx context.Context, principalID, fromTeamID, toTeamID int64, cost int) error {
	// Принципал привязан к игроку через Player.TeamPrincipal; здесь фиксируем владельца.
	groupID, err := d.resolveGroup(ctx, toTeamID)
	if err != nil {
		return err
	}
	p, err := d.GetPlayer(ctx, toTeamID, groupID)
	if err != nil {
		return err
	}
	id := principalID
	p.TeamPrincipal = &id
	return d.setJSON(ctx, playerKey(groupID, toTeamID), p)
}

// --- cross-season ---

func (d *Dynamic) ResetTokensAndBudget(ctx context.Context, groupID int64) error {
	players, err := d.GetPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, p := range players {
		p.Tokens = 120
		if err := d.setJSON(ctx, playerKey(groupID, p.ID), p); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dynamic) UpgradeTeam(ctx context.Context, groupID int64, team models.Team) error {
	return d.setJSON(ctx, teamKey(groupID, team.ID), team)
}

// --- groups ---

func (d *Dynamic) GetUserGroup(ctx context.Context, userID int64) (*int64, error) {
	g, err := d.rdb.Get(ctx, userGroupKey(userID)).Int64()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (d *Dynamic) GetGroupSize(ctx context.Context, groupID int64) (int, error) {
	n, err := d.rdb.SCard(ctx, playersIdx(groupID)).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (d *Dynamic) RegisterGroup(ctx context.Context, userID int64, name, password string) error {
	// groupID = userID организатора (простая детерминированная схема).
	groupID := userID
	meta := map[string]string{"name": name, "password": password}
	if err := d.setJSON(ctx, groupMetaKey(groupID), meta); err != nil {
		return err
	}
	if err := d.rdb.Set(ctx, groupByNameKey(name), groupID, 0).Err(); err != nil {
		return err
	}
	return d.SavePlayer(ctx, groupID, models.Player{ID: userID})
}

func (d *Dynamic) JoinGroup(ctx context.Context, userID int64, groupID int64, password string) error {
	var meta map[string]string
	ok, err := d.getJSON(ctx, groupMetaKey(groupID), &meta)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if meta["password"] != password {
		return errors.New("invalid group password")
	}
	return d.SavePlayer(ctx, groupID, models.Player{ID: userID})
}

// --- draft ---

func (d *Dynamic) GetPilotByGroup(ctx context.Context, pilotID, groupID int64) (models.Pilot, error) {
	var p models.Pilot
	ok, err := d.getJSON(ctx, pilotKey(groupID, pilotID), &p)
	if err != nil {
		return models.Pilot{}, err
	}
	if !ok {
		return models.Pilot{}, ErrNotFound
	}
	return p, nil
}

func (d *Dynamic) GetPlayerPilots(ctx context.Context, userID, groupID int64) ([]models.Pilot, error) {
	all, err := d.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []models.Pilot
	for _, p := range all {
		if p.Team != nil && *p.Team == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Dynamic) GetUnassignedPilots(ctx context.Context, groupID int64) ([]models.Pilot, error) {
	all, err := d.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []models.Pilot
	for _, p := range all {
		if p.Team == nil && (p.Garage == nil || *p.Garage == 0) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Dynamic) GetBotTeams(ctx context.Context, groupID int64) ([]models.Team, error) {
	players, err := d.GetPlayers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	owned := map[int64]bool{}
	for _, p := range players {
		if p.Team != 0 {
			owned[p.Team] = true
		}
	}
	teams, err := d.GetTeamsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []models.Team
	for _, t := range teams {
		if !owned[t.ID] {
			out = append(out, t)
		}
	}
	return out, nil
}

func (d *Dynamic) GetPilotsByTeam(ctx context.Context, teamID, groupID int64) ([]models.Pilot, error) {
	all, err := d.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []models.Pilot
	for _, p := range all {
		if p.Garage != nil && *p.Garage == teamID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *Dynamic) SetPlayerTeam(ctx context.Context, userID, groupID, teamID int64) error {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}
	p.Team = teamID
	return d.setJSON(ctx, playerKey(groupID, userID), p)
}

func (d *Dynamic) SetPlayerBudget(ctx context.Context, userID, groupID int64, budget int) error {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}
	p.Budget = budget
	return d.setJSON(ctx, playerKey(groupID, userID), p)
}

func (d *Dynamic) SetPlayerPrincipal(ctx context.Context, userID, groupID, principalID int64) error {
	p, err := d.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}
	id := principalID
	p.TeamPrincipal = &id
	return d.setJSON(ctx, playerKey(groupID, userID), p)
}

func (d *Dynamic) SetPilotOwner(ctx context.Context, pilotID, groupID int64, owner *int64, garage *int64) error {
	var p models.Pilot
	ok, err := d.getJSON(ctx, pilotKey(groupID, pilotID), &p)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	p.Team = copyPtr(owner)
	p.Garage = copyPtr(garage)
	return d.setJSON(ctx, pilotKey(groupID, pilotID), p)
}

func (d *Dynamic) SetTeamEngine(ctx context.Context, teamID, groupID int64, ice models.ICEName) error {
	t, err := d.GetTeamByGroup(ctx, teamID, groupID)
	if err != nil {
		return err
	}
	t.ICE = ice
	return d.setJSON(ctx, teamKey(groupID, teamID), t)
}

// --- engine.Repo ---

func (d *Dynamic) GetPilotTrack(ctx context.Context, groupID, pilotID, trackID int64) (models.PilotTrack, error) {
	var pt models.PilotTrack
	ok, err := d.getJSON(ctx, ptKey(groupID, pilotID, trackID), &pt)
	if err != nil {
		return models.PilotTrack{}, err
	}
	if !ok {
		return models.PilotTrack{PilotID: pilotID, TrackID: trackID}, nil
	}
	return pt, nil
}

func (d *Dynamic) UpdatePilot(ctx context.Context, groupID int64, pilot models.Pilot) error {
	return d.SavePilot(ctx, groupID, pilot)
}

func (d *Dynamic) UpdatePilotTrack(ctx context.Context, groupID int64, pt models.PilotTrack) error {
	return d.setJSON(ctx, ptKey(groupID, pt.PilotID, pt.TrackID), pt)
}

// --- helpers ---

func (d *Dynamic) resolveGroup(ctx context.Context, userID int64) (int64, error) {
	g, err := d.GetUserGroup(ctx, userID)
	if err != nil {
		return 0, err
	}
	if g == nil {
		return 0, ErrNotFound
	}
	return *g, nil
}

func copyPtr(p *int64) *int64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
