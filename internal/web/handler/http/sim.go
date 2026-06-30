package http

import (
	"context"
	"f1/internal/web/dto"
)

type Sim interface {
	MakeUpdate(ctx context.Context, userID int64, req dto.Updates) error
	ChooseSetup(ctx context.Context, userID int64, setup string) error
}
