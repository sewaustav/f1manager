package http

import (
	"context"
	"f1/internal/models"
)

type Data interface {
	GetPilotsService(ctx context.Context) ([]models.Pilot, error)
	GetTeamsService(ctx context.Context) ([]models.Team, error)
	GetPrincipalsService(ctx context.Context) ([]models.TeamPrincipal, error)
	GetTrackInfoService(ctx context.Context, track string) ([]models.Track, error)
	GetMyTeamService(ctx context.Context, userID int64) (models.MyTeam, error)
	GetPlayersService(ctx context.Context) ([]models.Player, error)
	GetPlayersTeamsService(ctx context.Context) ([]models.MyTeam, error)
}
