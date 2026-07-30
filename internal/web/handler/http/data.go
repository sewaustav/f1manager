package http

import (
	"context"
	"f1/internal/models"
)

type Data interface {
	GetPilotsService(ctx context.Context) ([]models.Pilot, error)
	GetTeamsService(ctx context.Context, userID int64) ([]models.Team, error)
	GetPrincipalsService(ctx context.Context) ([]models.TeamPrincipal, error)
	GetTrackInfoService(ctx context.Context, track string) ([]models.Track, error)
	GetMyTeamService(ctx context.Context, userID int64) (models.MyTeam, error)
	GetPlayersService(ctx context.Context, userID int64) ([]models.Player, error)
	GetPlayersTeamsService(ctx context.Context, userID int64) ([]models.MyTeam, error)
	GetEnginesService(ctx context.Context) ([]models.Engine, error)
	GetBudgetService(ctx context.Context, userID, groupID int64) (int, int, error)
}
