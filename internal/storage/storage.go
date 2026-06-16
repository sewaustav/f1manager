package storage

import (
	"context"
	"database/sql"
	"f1/internal/models"
)

type F1Repo interface {
	Begin(ctx context.Context) (Tx, error)
	WithTx(tx Tx) F1Repo
	
	GetPlayer(ctx context.Context, id int64) (models.PlayerProfile, error)
	GetPlayers(ctx context.Context) ([]models.PlayerProfile, error)
	GetTeam(ctx context.Context, teamID int64) (models.Team, error)
	GetTeams(ctx context.Context) ([]models.Team, error)
	GetPilots(ctx context.Context) ([]models.Pilot, error)
	GetTracks(ctx context.Context) ([]models.Track, error)
	GetPilot(ctx context.Context, id int64) (models.Pilot, error)
	GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error)
	GetTeamPrincipals(ctx context.Context) ([]models.TeamPrincipal, error)
	SavePlayer(ctx context.Context, player models.Player) (int64, error)
	UpdateTeamTokensAndBudget(ctx context.Context, teamID int64, tokens, budget int) error
	UpdateCar(ctx context.Context, car models.Car) error
	ExecuteTransfer(ctx context.Context, pilotID, fromTeamID, teamID int64, cost int) error
	TeamPrincipalTransfer(ctx context.Context, teamPrincipalID, fromTeamID, TeamID int64, cost int) error
	ResetSession(ctx context.Context) error
	GetBudget(ctx context.Context, teamID int64) (int, error)
	UpdateBudget(ctx context.Context, playerID int64, cost int) error
	GetTokens(ctx context.Context, playerID int64) (int, error)
	UpdateTokens(ctx context.Context, playerID int64, tokens int) error
	UpdatePilot(ctx context.Context, pilot models.Pilot) error
	UpdatePilotTrack(ctx context.Context, pt models.PilotTrack) error
	CreatePilots(ctx context.Context) error
	CreateTeams(ctx context.Context) ([]models.Team, error)
	GetActivePilots(ctx context.Context) ([]models.Pilot, error)
	UpdateTeam(ctx context.Context, team models.Team) error
	GetEngines(ctx context.Context) ([]models.Engine, error)
	Fire(ctx context.Context, userID, pilotID int64, who string) error
}

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

type Tx interface {
	DBTX
	Commit() error
	Rollback() error
}