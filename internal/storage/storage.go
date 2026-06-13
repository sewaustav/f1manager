package storage

import (
	"context"
	"f1/internal/models"
)

type F1Repo interface {
	GetTeams(ctx context.Context) ([]models.Team, error)
	GetPilots(ctx context.Context) ([]models.Pilot, error)
	GetTracks(ctx context.Context) ([]models.Track, error)
	GetPilot(ctx context.Context, id int64) (models.Pilot, error)
	GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error)
	SavePlayer(ctx context.Context, player models.Player) error
	UpdateTeamTokensAndBudget(ctx context.Context, teamID int64, tokens, budget int) error
	UpdateCar(ctx context.Context, car models.Car) error
	ExecuteTransfer(ctx context.Context, pilotID, fromTeamID, teamID int64, cost int) error
	ResetSession(ctx context.Context) error
	GetBudget(ctx context.Context, teamID int64) (int, error)
	UpdatePilot(ctx context.Context, pilot models.Pilot) error
	UpdatePilotTrack(ctx context.Context, pt models.PilotTrack) error
	CreatePilot(ctx context.Context, pilot models.Pilot, pilotTrack models.PilotTrack) error
	
}