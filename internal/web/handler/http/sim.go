package http

import (
	"context"
	"f1/internal/models"
	"f1/internal/web/dto"
)

type Sim interface {
	GetStanding(ctx context.Context, groupID int64) (map[int64]int, map[int64]int, error)
	GetLastRaceResults(ctx context.Context, groupID int64) ([]models.RaceResult, int64, error)
}

// SetupDispatcher принимает сетапы от игроков и запускает симуляцию,
// когда все участники группы прислали свои настройки.
type SetupDispatcher interface {
	Submit(ctx context.Context, userID, groupID int64, setup dto.Setup) error
	InitRound(groupID, stage int64, totalPlayers int)
}
