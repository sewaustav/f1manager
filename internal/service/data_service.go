package service

import (
	"context"
	"errors"

	"f1/internal/models"
)

// ErrNeedsGroupContext — метод требует groupID, которого нет в текущей сигнатуре Data-интерфейса.
// Эти эндпоинты будут дополнены после проброса группы в HTTP-слой.
var ErrNeedsGroupContext = errors.New("service: method needs group context")

func (s *Service) GetPilotsService(ctx context.Context) ([]models.Pilot, error) {
	return s.static.GetPilots(ctx)
}

func (s *Service) GetPrincipalsService(ctx context.Context) ([]models.TeamPrincipal, error) {
	return s.static.GetTeamPrincipals(ctx)
}

// GetTrackInfoService возвращает все трассы или конкретную по имени.
func (s *Service) GetTrackInfoService(ctx context.Context, track string) ([]models.Track, error) {
	tracks, err := s.static.GetTracks(ctx)
	if err != nil {
		return nil, err
	}
	if track == "" {
		return tracks, nil
	}
	var out []models.Track
	for _, t := range tracks {
		if t.Name == track {
			out = append(out, t)
		}
	}
	return out, nil
}

// GetMyTeamService собирает команду игрока: команда, два пилота, тим-принципал.
func (s *Service) GetMyTeamService(ctx context.Context, userID int64) (models.MyTeam, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return models.MyTeam{}, err
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return models.MyTeam{}, err
	}

	team, err := s.dynamic.GetTeamByGroup(ctx, player.Team, groupID)
	if err != nil {
		return models.MyTeam{}, err
	}

	pilots, err := s.dynamic.GetPlayerPilots(ctx, userID, groupID)
	if err != nil {
		return models.MyTeam{}, err
	}

	mt := models.MyTeam{ID: player.ID, Team: team}
	if len(pilots) > 0 {
		mt.Pilot1 = pilots[0]
	}
	if len(pilots) > 1 {
		mt.Pilot2 = pilots[1]
	}
	if player.TeamPrincipal != nil {
		if pr, err := s.static.GetTeamPrincipal(ctx, *player.TeamPrincipal); err == nil {
			mt.TeamPrincipal = pr
		}
	}
	return mt, nil
}

// GetTeamsService — команды скоупятся по группе; без groupID в сигнатуре не реализуемо.
func (s *Service) GetTeamsService(ctx context.Context) ([]models.Team, error) {
	return nil, ErrNeedsGroupContext
}

// GetPlayersService — игроки скоупятся по группе; без groupID не реализуемо.
func (s *Service) GetPlayersService(ctx context.Context) ([]models.Player, error) {
	return nil, ErrNeedsGroupContext
}

// GetPlayersTeamsService — составы скоупятся по группе; без groupID не реализуемо.
func (s *Service) GetPlayersTeamsService(ctx context.Context) ([]models.MyTeam, error) {
	return nil, ErrNeedsGroupContext
}
