package http

import (
	"context"
	"f1/internal/web/connection"
	"f1/internal/web/dto"
	ws "f1/internal/web/handler/websocket"
)

type User interface {
	GetUserGroup(ctx context.Context, userID int64) (*int64, error)
	RegisterGroup(ctx context.Context, userID int64, group dto.Group) error
	JoinGroup(ctx context.Context, userID int64, group dto.Group) error
}

type Manager interface {
	Register(userID, groupID int64, conn ws.Conn) *connection.Session
	GroupSize(groupID int64) int
}
