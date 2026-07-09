package service

import (
	"context"
	"errors"
	"sort"
)

// AutoFillAfterDraft раздаёт свободных пилотов (garage=null) командам,
// которым не хватает до 2 пилотов, по убыванию рейтинга.
func (s *Service) AutoFillAfterDraft(ctx context.Context, groupID int64) error {
	free, err := s.dynamic.GetUnassignedPilots(ctx, groupID)
	if err != nil {
		return err
	}
	sort.Slice(free, func(i, j int) bool { return free[i].Rating > free[j].Rating })

	teams, err := s.dynamic.GetTeamsByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].ID < teams[j].ID })

	idx := 0
	for _, t := range teams {
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
