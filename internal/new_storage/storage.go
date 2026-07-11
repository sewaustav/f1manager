package repo

import (
	"context"
	"f1/internal/models"
)

// StaticRepo содержит данные, которые не изменяются в ходе игры.
type StaticRepo interface {
	GetPilot(ctx context.Context, pilotID int64) (models.Pilot, error)
	GetPilots(ctx context.Context) ([]models.Pilot, error)
	GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error)

	GetTrack(ctx context.Context, trackID int64) (models.Track, error)
	GetTracks(ctx context.Context) ([]models.Track, error)

	GetTeamPrincipal(ctx context.Context, principalID int64) (models.TeamPrincipal, error)
	GetTeamPrincipals(ctx context.Context) ([]models.TeamPrincipal, error)

	GetEngine(ctx context.Context, id int64) (models.Engine, error)
	GetEngines(ctx context.Context) ([]models.Engine, error)
}

// DynamicRepo содержит данные, которые изменяются в ходе игры.
type DynamicRepo interface {
	// Чтение
	GetPlayer(ctx context.Context, userID, groupID int64) (models.Player, error)
	GetPlayers(ctx context.Context, groupID int64) ([]models.Player, error)
	GetPilotsByGroup(ctx context.Context, groupID int64) ([]models.Pilot, error)
	GetTeamsByGroup(ctx context.Context, groupID int64) ([]models.Team, error)
	GetTeamByGroup(ctx context.Context, teamID, groupID int64) (models.Team, error)
	GetCar(ctx context.Context, teamID, groupID int64) (models.Car, error)
	GetBudget(ctx context.Context, userID, groupID int64) (int, error)
	GetTokens(ctx context.Context, userID, groupID int64) (int, error)
	GetStanding(ctx context.Context, groupID int64) (driverPoints map[int64]int, teamPoints map[int64]int, err error)
	GetLastRaceResults(ctx context.Context, groupID int64) ([]models.RaceResult, int64, error)

	// Результаты гонки
	HandleRace(ctx context.Context, race []models.RaceResult, groupID int64) error

	// Обновление состояния
	UpdateCar(ctx context.Context, teamID, groupID int64, car models.Car) error
	UpdateTeam(ctx context.Context, userID int64, team models.Team) error
	UpdatePlayer(ctx context.Context, userID, groupID int64, player models.Player) error
	UpdateBudget(ctx context.Context, userID, groupID int64, delta int) error
	UpdateTokens(ctx context.Context, userID, groupID int64, tokens int) error

	// Трансферы
	ExecutePilotTransfer(ctx context.Context, pilotID, fromTeamID, toTeamID int64, cost int) error
	ExecutePrincipalTransfer(ctx context.Context, principalID, fromTeamID, toTeamID int64, cost int) error

	// Межсезонье
	ResetTokensAndBudget(ctx context.Context, groupID int64) error
	UpgradeTeam(ctx context.Context, groupID int64, team models.Team) error

	// Группы/игроки
	GetUserGroup(ctx context.Context, userID int64) (*int64, error)
	GetGroupSize(ctx context.Context, groupID int64) (int, error)
	RegisterGroup(ctx context.Context, userID int64, name, password string) error
	JoinGroup(ctx context.Context, userID int64, groupID int64, password string) error

	// Драфт
	GetPilotByGroup(ctx context.Context, pilotID, groupID int64) (models.Pilot, error)
	GetPlayerPilots(ctx context.Context, userID, groupID int64) ([]models.Pilot, error)
	GetUnassignedPilots(ctx context.Context, groupID int64) ([]models.Pilot, error)
	GetBotTeams(ctx context.Context, groupID int64) ([]models.Team, error)
	GetPilotsByTeam(ctx context.Context, teamID, groupID int64) ([]models.Pilot, error)
	SetPlayerTeam(ctx context.Context, userID, groupID, teamID int64) error
	SetPlayerBudget(ctx context.Context, userID, groupID int64, budget int) error
	SetPlayerPrincipal(ctx context.Context, userID, groupID, principalID int64) error
	SetPilotOwner(ctx context.Context, pilotID, groupID int64, owner *int64, garage *int64) error
	SetTeamEngine(ctx context.Context, teamID, groupID int64, ice models.ICEName) error
}