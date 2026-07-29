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

	ice, engineCost, err := s.resolveEngine(ctx, team, pick.Engine)
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

// clientDefaultEngineCost — списание с клиента, не выбравшего мотор (по максимуму).
const clientDefaultEngineCost = 30

// resolveEngine определяет мотор команды и его стоимость:
//   - завод (Manufacture): всегда свой мотор по базовой цене (без +10);
//   - полу-завод (Semi): свой мотор — базовая цена; чужой — базовая +10; без выбора — свой;
//   - клиент (Client): выбранный мотор — базовая +10; без выбора — дефолтный не-топовый мотор
//     со списанием по максимуму (clientDefaultEngineCost).
func (s *Service) resolveEngine(ctx context.Context, team models.Team, chosen *models.ICEName) (models.ICEName, int, error) {
	engines, err := s.static.GetEngines(ctx)
	if err != nil {
		return 0, 0, err
	}
	priceOf := func(ice models.ICEName) (int, bool) {
		for _, e := range engines {
			if e.Engine == ice {
				return e.Price, true
			}
		}
		return 0, false
	}

	switch team.IsManufacturer {
	case models.Manufacture:
		p, ok := priceOf(team.ICE)
		if !ok {
			return 0, 0, errors.New("мотор не найден")
		}
		return team.ICE, p, nil

	case models.Semi:
		ice := team.ICE
		if chosen != nil {
			ice = *chosen
		}
		p, ok := priceOf(ice)
		if !ok {
			return 0, 0, errors.New("мотор не найден")
		}
		if ice != team.ICE {
			p += 10 // чужой мотор
		}
		return ice, p, nil

	default: // Client
		if chosen == nil {
			ice, ok := nonTopEngine(engines)
			if !ok {
				return 0, 0, errors.New("нет доступных моторов")
			}
			return ice, clientDefaultEngineCost, nil
		}
		p, ok := priceOf(*chosen)
		if !ok {
			return 0, 0, errors.New("мотор не найден")
		}
		return *chosen, p + 10, nil
	}
}

// nonTopEngine возвращает заведомо не-топовый мотор (самый слабый по BaseLevel).
func nonTopEngine(engines []models.Engine) (models.ICEName, bool) {
	if len(engines) == 0 {
		return 0, false
	}
	weakest := engines[0]
	for _, e := range engines {
		if e.BaseLevel < weakest.BaseLevel {
			weakest = e
		}
	}
	return weakest.Engine, true
}
