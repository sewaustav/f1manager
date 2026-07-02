package http

import (
	"context"
	"f1/internal/web/dto"
)

// CrossSeason — действия между этапами и в межсезонье.
type CrossSeason interface {
	MakeTokenSetup(ctx context.Context, userID int64, setup dto.Setup) error
	MakeUpdate(ctx context.Context, userID int64, req dto.Updates) error
	UpdateBase(ctx context.Context, userID int64, req dto.BaseUpdate) error
	PilotTransfer(ctx context.Context, userID int64, req dto.PilotTransfer) error
	PrincipalTransfer(ctx context.Context, userID int64, req dto.PrincipalTransfer) error
	ResetSeason(ctx context.Context, groupID int64) error
	PickItem(ctx context.Context, userID int64, item dto.DraftItem) error
}
