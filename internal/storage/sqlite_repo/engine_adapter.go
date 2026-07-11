package sqlite_repo

import (
	"context"

	"f1/internal/models"
)

// EngineAdapter адаптирует SQLite-репозиторий (без групп) под engine.Repo.
// groupID игнорируется — CLI работает с единым глобальным состоянием.
type EngineAdapter struct {
	repo *SqliteF1Repo
}

func NewEngineAdapter(r *SqliteF1Repo) *EngineAdapter {
	return &EngineAdapter{repo: r}
}

func (a *EngineAdapter) GetPilotTrack(ctx context.Context, _ int64, pilotID, trackID int64) (models.PilotTrack, error) {
	return a.repo.GetPilotTrack(ctx, pilotID, trackID)
}

func (a *EngineAdapter) UpdatePilot(ctx context.Context, _ int64, pilot models.Pilot) error {
	return a.repo.UpdatePilot(ctx, pilot)
}

func (a *EngineAdapter) UpdatePilotTrack(ctx context.Context, _ int64, pt models.PilotTrack) error {
	return a.repo.UpdatePilotTrack(ctx, pt)
}
