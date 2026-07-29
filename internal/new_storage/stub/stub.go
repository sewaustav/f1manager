// Package stub — временные заглушки StaticRepo/DynamicRepo,
// чтобы сервер собирался до появления Postgres-реализации игрового репозитория.
package stub

import (
	"context"
	"errors"

	"f1/internal/models"
	repo "f1/internal/new_storage"
)

var ErrNotImplemented = errors.New("storage: not implemented")

type Static struct{}

func NewStatic() *Static { return &Static{} }

var _ repo.StaticRepo = (*Static)(nil)

func (s *Static) GetPilot(ctx context.Context, pilotID int64) (models.Pilot, error) {
	return models.Pilot{}, ErrNotImplemented
}

func (s *Static) GetPilots(ctx context.Context) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

func (s *Static) GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error) {
	return models.PilotTrack{}, ErrNotImplemented
}

func (s *Static) GetTrack(ctx context.Context, trackID int64) (models.Track, error) {
	return models.Track{}, ErrNotImplemented
}

func (s *Static) GetTracks(ctx context.Context) ([]models.Track, error) {
	return nil, ErrNotImplemented
}

func (s *Static) GetTeamPrincipal(ctx context.Context, principalID int64) (models.TeamPrincipal, error) {
	return models.TeamPrincipal{}, ErrNotImplemented
}

func (s *Static) GetTeamPrincipals(ctx context.Context) ([]models.TeamPrincipal, error) {
	return nil, ErrNotImplemented
}

func (s *Static) GetEngine(ctx context.Context, id int64) (models.Engine, error) {
	return models.Engine{}, ErrNotImplemented
}

func (s *Static) GetEngines(ctx context.Context) ([]models.Engine, error) {
	return nil, ErrNotImplemented
}

type Dynamic struct{}

func NewDynamic() *Dynamic { return &Dynamic{} }

var _ repo.DynamicRepo = (*Dynamic)(nil)

func (d *Dynamic) GetPlayer(ctx context.Context, userID, groupID int64) (models.Player, error) {
	return models.Player{}, ErrNotImplemented
}

func (d *Dynamic) GetPlayers(ctx context.Context, groupID int64) ([]models.Player, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetPilotsByGroup(ctx context.Context, groupID int64) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetTeamsByGroup(ctx context.Context, groupID int64) ([]models.Team, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetTeamByGroup(ctx context.Context, teamID, groupID int64) (models.Team, error) {
	return models.Team{}, ErrNotImplemented
}

func (d *Dynamic) GetCar(ctx context.Context, teamID, groupID int64) (models.Car, error) {
	return models.Car{}, ErrNotImplemented
}

func (d *Dynamic) GetBudget(ctx context.Context, userID, groupID int64) (int, error) {
	return 0, ErrNotImplemented
}

func (d *Dynamic) GetTokens(ctx context.Context, userID, groupID int64) (int, error) {
	return 0, ErrNotImplemented
}

func (d *Dynamic) GetStanding(ctx context.Context, groupID int64) (map[int64]int, map[int64]int, error) {
	return nil, nil, ErrNotImplemented
}

func (d *Dynamic) GetLastRaceResults(ctx context.Context, groupID int64) ([]models.RaceResult, int64, error) {
	return nil, 0, ErrNotImplemented
}

func (d *Dynamic) HandleRace(ctx context.Context, race []models.RaceResult, groupID int64) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpdateCar(ctx context.Context, teamID, groupID int64, car models.Car) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpdateTeam(ctx context.Context, userID int64, team models.Team) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpdatePlayer(ctx context.Context, userID, groupID int64, player models.Player) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpdateBudget(ctx context.Context, userID, groupID int64, delta int) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpdateTokens(ctx context.Context, userID, groupID int64, tokens int) error {
	return ErrNotImplemented
}

func (d *Dynamic) ExecutePilotTransfer(ctx context.Context, pilotID, fromTeamID, toTeamID int64, cost int) error {
	return ErrNotImplemented
}

func (d *Dynamic) ExecutePrincipalTransfer(ctx context.Context, principalID, fromTeamID, toTeamID int64, cost int) error {
	return ErrNotImplemented
}

func (d *Dynamic) ResetTokensAndBudget(ctx context.Context, groupID int64) error {
	return ErrNotImplemented
}

func (d *Dynamic) UpgradeTeam(ctx context.Context, groupID int64, team models.Team) error {
	return ErrNotImplemented
}

func (d *Dynamic) GetUserGroup(ctx context.Context, userID int64) (*int64, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetGroupSize(ctx context.Context, groupID int64) (int, error) {
	return 0, ErrNotImplemented
}

func (d *Dynamic) RegisterGroup(ctx context.Context, userID int64, name, password string) error {
	return ErrNotImplemented
}

func (d *Dynamic) JoinGroup(ctx context.Context, userID int64, groupID int64, password string) error {
	return ErrNotImplemented
}

func (d *Dynamic) GetPilotByGroup(ctx context.Context, pilotID, groupID int64) (models.Pilot, error) {
	return models.Pilot{}, ErrNotImplemented
}

func (d *Dynamic) GetPlayerPilots(ctx context.Context, userID, groupID int64) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetUnassignedPilots(ctx context.Context, groupID int64) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetBotTeams(ctx context.Context, groupID int64) ([]models.Team, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) GetPilotsByTeam(ctx context.Context, teamID, groupID int64) ([]models.Pilot, error) {
	return nil, ErrNotImplemented
}

func (d *Dynamic) SetPlayerTeam(ctx context.Context, userID, groupID, teamID int64) error {
	return ErrNotImplemented
}

func (d *Dynamic) SetPlayerBudget(ctx context.Context, userID, groupID int64, budget int) error {
	return ErrNotImplemented
}

func (d *Dynamic) SetPlayerPrincipal(ctx context.Context, userID, groupID, principalID int64) error {
	return ErrNotImplemented
}

func (d *Dynamic) SetPilotOwner(ctx context.Context, pilotID, groupID int64, owner *int64, garage *int64) error {
	return ErrNotImplemented
}

func (d *Dynamic) SetTeamEngine(ctx context.Context, teamID, groupID int64, ice models.ICEName) error {
	return ErrNotImplemented
}

// EngineRepo — заглушка engine.Repo для сборки сервера до появления Redis-реализации.
type EngineRepo struct{}

func NewEngineRepo() *EngineRepo { return &EngineRepo{} }

func (e *EngineRepo) GetPilotTrack(ctx context.Context, groupID, pilotID, trackID int64) (models.PilotTrack, error) {
	return models.PilotTrack{}, ErrNotImplemented
}

func (e *EngineRepo) UpdatePilot(ctx context.Context, groupID int64, pilot models.Pilot) error {
	return ErrNotImplemented
}

func (e *EngineRepo) UpdatePilotTrack(ctx context.Context, groupID int64, pt models.PilotTrack) error {
	return ErrNotImplemented
}
