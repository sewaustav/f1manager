package service

import (
	"context"
	"errors"

	"f1/internal/models"
	"f1/internal/web/dto"
)

const draftVirtualBudget = 110

// StartDraftEconomy инициализирует виртуальный бюджет каждого игрока перед драфтом.
func (s *Service) StartDraftEconomy(ctx context.Context, groupID int64, players []int64) error {
	for _, uid := range players {
		if err := s.dynamic.SetPlayerBudget(ctx, uid, groupID, draftVirtualBudget); err != nil {
			return err
		}
	}
	return nil
}

// ListGroupPlayers возвращает id всех игроков группы.
func (s *Service) ListGroupPlayers(ctx context.Context, groupID int64) ([]int64, error) {
	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
	}
	return ids, nil
}

// ApplyDraftPick применяет один пик игрока с валидацией лимитов, доступности и бюджета.
func (s *Service) ApplyDraftPick(ctx context.Context, userID, groupID int64, pick dto.Draft) error {
	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	switch pick.Pick {
	case dto.DraftTeam:
		return s.applyTeamPick(ctx, userID, groupID, player, pick)
	case dto.DraftPilot:
		return s.applyPilotPick(ctx, userID, groupID, player, pick)
	case dto.DraftPrincipal:
		return s.applyPrincipalPick(ctx, userID, groupID, player, pick)
	default:
		return errors.New("неизвестный тип пика")
	}
}

func (s *Service) applyTeamPick(ctx context.Context, userID, groupID int64, player models.Player, pick dto.Draft) error {
	if player.Team != 0 {
		return errors.New("команда уже выбрана")
	}
	if pick.Engine == nil {
		return errors.New("для команды нужно указать мотор")
	}

	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, p := range players {
		if p.Team == pick.ItemID {
			return errors.New("команда уже занята")
		}
	}

	team, err := s.dynamic.GetTeamByGroup(ctx, pick.ItemID, groupID)
	if err != nil {
		return err
	}

	ice := *pick.Engine
	if team.IsManufacturer == models.Manufacture {
		ice = team.ICE // заводу мотор принудительно свой
	}

	engineCost, err := s.engineCost(ctx, ice, team.IsManufacturer)
	if err != nil {
		return err
	}

	currentBudget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}
	spent := draftVirtualBudget - currentBudget
	newBudget := team.Budget - spent - engineCost
	if newBudget < 0 {
		return errors.New("недостаточно бюджета для команды")
	}

	if err := s.dynamic.SetPlayerTeam(ctx, userID, groupID, pick.ItemID); err != nil {
		return err
	}
	if err := s.dynamic.SetTeamEngine(ctx, pick.ItemID, groupID, ice); err != nil {
		return err
	}
	if err := s.dynamic.SetPlayerBudget(ctx, userID, groupID, newBudget); err != nil {
		return err
	}

	// back-fill гаража для пилотов, взятых до команды
	owned, err := s.dynamic.GetPlayerPilots(ctx, userID, groupID)
	if err != nil {
		return err
	}
	teamID := pick.ItemID
	for _, pl := range owned {
		if err := s.dynamic.SetPilotOwner(ctx, pl.ID, groupID, &userID, &teamID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) applyPilotPick(ctx context.Context, userID, groupID int64, player models.Player, pick dto.Draft) error {
	owned, err := s.dynamic.GetPlayerPilots(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if len(owned) >= 2 {
		return errors.New("уже выбрано 2 пилота")
	}

	pilot, err := s.dynamic.GetPilotByGroup(ctx, pick.ItemID, groupID)
	if err != nil {
		return err
	}
	// Доступность = нет владельца-игрока (team_id). Дефолтный garage_id из
	// pilots_initial не блокирует драфт — он лишь стартовая команда пилота.
	if pilot.Team != nil {
		return errors.New("пилот уже занят")
	}

	cost := pilot.Price - pilot.Sponsors
	currentBudget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if currentBudget < cost {
		return errors.New("недостаточно бюджета для пилота")
	}

	var garage *int64
	if player.Team != 0 {
		t := player.Team
		garage = &t
	}
	if err := s.dynamic.SetPilotOwner(ctx, pick.ItemID, groupID, &userID, garage); err != nil {
		return err
	}
	return s.dynamic.SetPlayerBudget(ctx, userID, groupID, currentBudget-cost)
}

func (s *Service) applyPrincipalPick(ctx context.Context, userID, groupID int64, player models.Player, pick dto.Draft) error {
	if player.TeamPrincipal != nil {
		return errors.New("тим-принципал уже выбран")
	}

	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, p := range players {
		if p.TeamPrincipal != nil && *p.TeamPrincipal == pick.ItemID {
			return errors.New("тим-принципал уже занят")
		}
	}

	principal, err := s.static.GetTeamPrincipal(ctx, pick.ItemID)
	if err != nil {
		return err
	}

	currentBudget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if currentBudget < principal.Price {
		return errors.New("недостаточно бюджета для тим-принципала")
	}

	if err := s.dynamic.SetPlayerPrincipal(ctx, userID, groupID, pick.ItemID); err != nil {
		return err
	}
	return s.dynamic.SetPlayerBudget(ctx, userID, groupID, currentBudget-principal.Price)
}

// engineCost — стоимость мотора: завод платит price, остальные price+10.
func (s *Service) engineCost(ctx context.Context, ice models.ICEName, kind models.IsManufacturer) (int, error) {
	engines, err := s.static.GetEngines(ctx)
	if err != nil {
		return 0, err
	}
	for _, e := range engines {
		if e.Engine == ice {
			if kind == models.Manufacture {
				return e.Price, nil
			}
			return e.Price + 10, nil
		}
	}
	return 0, errors.New("мотор не найден")
}
