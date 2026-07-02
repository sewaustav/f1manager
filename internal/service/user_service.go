package service

import (
	"context"
	"f1/internal/web/dto"
)

func (s *Service) GetUserGroup(ctx context.Context, userID int64) (*int64, error) {
	return s.dynamic.GetUserGroup(ctx, userID)
}

func (s *Service) RegisterGroup(ctx context.Context, userID int64, group dto.Group) error {
	return s.dynamic.RegisterGroup(ctx, userID, group.Name, group.Password)
}

func (s *Service) JoinGroup(ctx context.Context, userID int64, group dto.Group) error {
	return s.dynamic.JoinGroup(ctx, userID, group.ID, group.Password)
}
