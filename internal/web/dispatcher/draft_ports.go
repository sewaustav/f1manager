package dispatcher

import (
	"context"

	"f1/internal/web/dto"
)

// DraftService — бизнес-логика драфта, которую использует диспетчер.
type DraftService interface {
	ListGroupPlayers(ctx context.Context, groupID int64) ([]int64, error)
	StartDraftEconomy(ctx context.Context, groupID int64, players []int64) error
	ApplyDraftPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error
	AutoFillAfterDraft(ctx context.Context, groupID int64) error
}

// DraftNotifier — WS-уведомления участникам группы.
type DraftNotifier interface {
	SendUser(userID int64, msg []byte)
	BroadcastGroup(groupID int64, msg []byte)
}
