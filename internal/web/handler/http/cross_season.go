package http

import (
	"context"
	"f1/internal/web/dto"
)

type CrossSeason interface {
	MakeTokenSetup(ctx context.Context, userID int64, req dto.Setup) error 
	UpdateBase(ctx context.Context, userID int64, req dto.BaseUpdate) error
	PickItem(ctx context.Context, userID int64, req dto.DraftItem) error
}
