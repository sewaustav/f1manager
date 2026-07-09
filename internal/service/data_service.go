package service

import (
	"context"
	"errors"

	"f1/internal/models"
)

var ErrNotImplemented = errors.New("service: not implemented")

func (s *Service) GetPilotsService(ctx context.Context) ([]models.Pilot, error) {
	return s.static.GetPilots(ctx)
}

func (s *Service) GetTeamsService(ctx context.Context) ([]models.Team, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetPrincipalsService(ctx context.Context) ([]models.TeamPrincipal, error) {
	return s.static.GetTeamPrincipals(ctx)
}

func (s *Service) GetTrackInfoService(ctx context.Context, track string) ([]models.Track, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetMyTeamService(ctx context.Context, userID int64) (models.MyTeam, error) {
	return models.MyTeam{}, ErrNotImplemented
}

func (s *Service) GetPlayersService(ctx context.Context) ([]models.Player, error) {
	return nil, ErrNotImplemented
}

func (s *Service) GetPlayersTeamsService(ctx context.Context) ([]models.MyTeam, error) {
	return nil, ErrNotImplemented
}
