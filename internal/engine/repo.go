package engine

import (
	"context"

	"f1/internal/models"
)

// Repo — доступ к данным пилотов/трасс, нужный движку.
// Все операции скоупятся по groupID (групповая изоляция веб-версии);
// реализации без групп (CLI/SQLite) игнорируют groupID.
type Repo interface {
	GetPilotTrack(ctx context.Context, groupID, pilotID, trackID int64) (models.PilotTrack, error)
	UpdatePilot(ctx context.Context, groupID int64, pilot models.Pilot) error
	UpdatePilotTrack(ctx context.Context, groupID int64, pt models.PilotTrack) error
}
