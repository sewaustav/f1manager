package service

import (
	"context"

	"f1/internal/models"
)

// GetPilotsService возвращает пилотов группы игрока с их текущим статусом
// драфта (Team != nil у уже выбранного пилота) — не голый статический
// список, который никогда не отражал бы, кто уже занят.
func (s *Service) GetPilotsService(ctx context.Context, userID int64) ([]models.Pilot, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.dynamic.GetPilotsByGroup(ctx, groupID)
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

	return s.buildMyTeam(ctx, groupID, player)
}

// buildMyTeam собирает MyTeam для одного игрока. До того как игрок выберет
// команду на драфте, player.Team == 0 — в этом случае Team остаётся
// нулевым значением вместо похода в Redis за несуществующим ключом.
func (s *Service) buildMyTeam(ctx context.Context, groupID int64, player models.Player) (models.MyTeam, error) {
	mt := models.MyTeam{ID: player.ID}
	if player.Team != 0 {
		team, err := s.dynamic.GetTeamByGroup(ctx, player.Team, groupID)
		if err != nil {
			return models.MyTeam{}, err
		}
		mt.Team = team
	}

	pilots, err := s.dynamic.GetPlayerPilots(ctx, player.ID, groupID)
	if err != nil {
		return models.MyTeam{}, err
	}

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

// GetTeamsService возвращает все команды группы игрока.
func (s *Service) GetTeamsService(ctx context.Context, userID int64) ([]models.Team, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.dynamic.GetTeamsByGroup(ctx, groupID)
}

// GetPlayersService возвращает всех игроков группы.
func (s *Service) GetPlayersService(ctx context.Context, userID int64) ([]models.Player, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.dynamic.GetPlayers(ctx, groupID)
}

// GetPlayersTeamsService возвращает составы всех игроков группы.
func (s *Service) GetPlayersTeamsService(ctx context.Context, userID int64) ([]models.MyTeam, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	squads := make([]models.MyTeam, 0, len(players))
	for _, p := range players {
		mt, err := s.buildMyTeam(ctx, groupID, p)
		if err != nil {
			return nil, err
		}
		squads = append(squads, mt)
	}
	return squads, nil
}

func (s *Service) GetEnginesService(ctx context.Context) ([]models.Engine, error) {
	return s.static.GetEngines(ctx)
}

// GetBudgetService returns (budget, tokens) for the player in the group.
func (s *Service) GetBudgetService(ctx context.Context, userID, groupID int64) (int, int, error) {
	budget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return 0, 0, err
	}
	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return 0, 0, err
	}
	return budget, player.Tokens, nil
}
