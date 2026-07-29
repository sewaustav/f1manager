package service

import (
	"context"
	"errors"
	"sort"

	"f1/internal/models"
)

// AutoFillAfterDraft приводит гаражи в порядок после драфта:
//  1. недобранные пилоты, чей дефолтный гараж — команда, доставшаяся игроку,
//     освобождаются (garage=0), т.к. игрок мог взять в эту команду других пилотов;
//  2. недобранные пилоты команд-ботов сохраняют дефолтный гараж из pilots_initial;
//  3. образовавшийся дефицит (<2 пилота) закрывается свободными пилотами по убыванию
//     рейтинга — сначала команды игроков, затем боты.
func (s *Service) AutoFillAfterDraft(ctx context.Context, groupID int64) error {
	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	playerTeams := map[int64]bool{}
	for _, p := range players {
		if p.Team != 0 {
			playerTeams[p.Team] = true
		}
	}

	all, err := s.dynamic.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return err
	}

	// 1. Освобождаем недобранных пилотов команд, доставшихся игрокам.
	for _, pl := range all {
		if pl.Team == nil && pl.Garage != nil && playerTeams[*pl.Garage] {
			if err := s.dynamic.SetPilotOwner(ctx, pl.ID, groupID, nil, nil); err != nil {
				return err
			}
		}
	}

	// 2. Пул свободных: без владельца-игрока и без гаража (0/nil).
	all, err = s.dynamic.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	var free []models.Pilot
	for _, pl := range all {
		if pl.Team == nil && (pl.Garage == nil || *pl.Garage == 0) {
			free = append(free, pl)
		}
	}
	sort.Slice(free, func(i, j int) bool { return free[i].Rating > free[j].Rating })

	// 3. Закрываем дефицит: сначала команды игроков, потом боты.
	teams, err := s.dynamic.GetTeamsByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	var playerFirst, bots []models.Team
	for _, t := range teams {
		if playerTeams[t.ID] {
			playerFirst = append(playerFirst, t)
		} else {
			bots = append(bots, t)
		}
	}
	sort.Slice(playerFirst, func(i, j int) bool { return playerFirst[i].ID < playerFirst[j].ID })
	sort.Slice(bots, func(i, j int) bool { return bots[i].ID < bots[j].ID })
	ordered := append(playerFirst, bots...)

	idx := 0
	for _, t := range ordered {
		pilots, err := s.dynamic.GetPilotsByTeam(ctx, t.ID, groupID)
		if err != nil {
			return err
		}
		count := len(pilots)
		for count < 2 && idx < len(free) {
			garage := t.ID
			if err := s.dynamic.SetPilotOwner(ctx, free[idx].ID, groupID, nil, &garage); err != nil {
				return err
			}
			idx++
			count++
		}
	}
	return nil
}

// SwapBotPilots меняет пилотов местами между двумя командами-ботами.
func (s *Service) SwapBotPilots(ctx context.Context, groupID, teamA, teamB, pilotA, pilotB int64) error {
	bots, err := s.dynamic.GetBotTeams(ctx, groupID)
	if err != nil {
		return err
	}
	isBot := func(id int64) bool {
		for _, b := range bots {
			if b.ID == id {
				return true
			}
		}
		return false
	}
	if !isBot(teamA) || !isBot(teamB) {
		return errors.New("менять можно только пилотов команд-ботов")
	}

	if err := s.pilotInTeam(ctx, groupID, pilotA, teamA); err != nil {
		return err
	}
	if err := s.pilotInTeam(ctx, groupID, pilotB, teamB); err != nil {
		return err
	}

	a := teamA
	b := teamB
	if err := s.dynamic.SetPilotOwner(ctx, pilotA, groupID, nil, &b); err != nil {
		return err
	}
	return s.dynamic.SetPilotOwner(ctx, pilotB, groupID, nil, &a)
}

func (s *Service) pilotInTeam(ctx context.Context, groupID, pilotID, teamID int64) error {
	pilots, err := s.dynamic.GetPilotsByTeam(ctx, teamID, groupID)
	if err != nil {
		return err
	}
	for _, p := range pilots {
		if p.ID == pilotID {
			return nil
		}
	}
	return errors.New("пилот не принадлежит указанной команде")
}
