package service

import (
	"context"
)

type TypeUpdate int

const (
	Car TypeUpdate = iota
	Synergy
)

type Update struct {
	Key      string
	TeamID   int64
	GroupID  int64
	PlayerID int64
	Stage    int64
	Bonus    int
	Type     TypeUpdate
}

type UpdateCache interface {
	PutUpdate(ctx context.Context, update Update) error
	GetUpdates(ctx context.Context, groupID int64) ([]Update, error)
	DeleteUpdate(ctx context.Context, key string) error
}
