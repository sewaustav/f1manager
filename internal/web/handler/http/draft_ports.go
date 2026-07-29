package http

import (
	"context"

	"f1/internal/web/dto"
)

// draftDispatcher — пошаговый драфт, которым управляет handler.
type draftDispatcher interface {
	StartDraft(ctx context.Context, groupID int64) error
	SubmitPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error
}

// draftService — операции сервиса, нужные draft-handler'у.
type draftService interface {
	GetUserGroup(ctx context.Context, userID int64) (*int64, error)
	SwapBotPilots(ctx context.Context, groupID, teamA, teamB, pilotA, pilotB int64) error
}
