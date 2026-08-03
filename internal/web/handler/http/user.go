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

	// ResetGroup wipes a group's gameplay data back to a fresh pre-draft
	// lobby — see POST /groups/reset ("end the game early").
	ResetGroup(ctx context.Context, groupID int64) error

	// LeaveGroup выводит игрока из группы (POST /groups/leave).
	LeaveGroup(ctx context.Context, userID int64) error

	// KickPlayer — организатор удаляет участника (POST /groups/kick).
	KickPlayer(ctx context.Context, organizerID, targetID int64) error
}

type Manager interface {
	Register(userID, groupID int64, conn *ws.Conn) *connection.Session
	GroupSize(groupID int64) int
	BroadcastGroup(groupID int64, msg []byte)
}
