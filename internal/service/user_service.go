package service

import (
	"context"
	"errors"

	"f1/internal/models"
	"f1/internal/web/dto"
)

func (s *Service) GetUserGroup(ctx context.Context, userID int64) (*int64, error) {
	return s.dynamic.GetUserGroup(ctx, userID)
}

func (s *Service) RegisterGroup(ctx context.Context, userID int64, group dto.Group) error {
	// Идентификатор группы — это id организатора, поэтому повторное создание
	// попадает в тот же ключ. Без очистки в «новую» группу подтягивались
	// участники прошлой сессии со всем их состоянием.
	if err := s.dynamic.ClearGroup(ctx, userID); err != nil {
		return err
	}
	if err := s.dynamic.RegisterGroup(ctx, userID, group.Name, group.Password); err != nil {
		return err
	}
	return s.seedGroupWorld(ctx, userID)
}

// KickPlayer — удаление участника организатором. Себя выгнать нельзя:
// организатору для выхода есть LeaveGroup, а для сброса игры — ResetGroup.
func (s *Service) KickPlayer(ctx context.Context, organizerID, targetID int64) error {
	groupID, err := s.getUserGroup(ctx, organizerID)
	if err != nil {
		return err
	}
	if organizerID != groupID {
		return errors.New("удалять участников может только организатор")
	}
	if targetID == organizerID {
		return errors.New("организатор не может удалить самого себя")
	}
	targetGroup, err := s.dynamic.GetUserGroup(ctx, targetID)
	if err != nil {
		return err
	}
	if targetGroup == nil || *targetGroup != groupID {
		return errors.New("игрок не состоит в вашей группе")
	}
	return s.LeaveGroup(ctx, targetID)
}

// seedGroupWorld копирует статические шаблоны команд (base_team) и пилотов
// (pilots_initial) в изолированный мир новой группы. Без этого шага
// SetPilotOwner/SetTeamEngine/GetTeamByGroup/GetPilotByGroup — то есть весь
// драфт — падают с ErrNotFound на любой свежесозданной группе: до этого
// исправления сюда никто не писал ни для одной группы вообще.
func (s *Service) seedGroupWorld(ctx context.Context, groupID int64) error {
	teams, err := s.static.GetBaseTeams(ctx)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err := s.dynamic.SaveTeam(ctx, groupID, t); err != nil {
			return err
		}
	}

	pilots, err := s.static.GetPilots(ctx)
	if err != nil {
		return err
	}
	for _, p := range pilots {
		if err := s.dynamic.SavePilot(ctx, groupID, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) JoinGroup(ctx context.Context, userID int64, group dto.Group) error {
	return s.dynamic.JoinGroup(ctx, userID, group.ID, group.Password)
}

// ResetGroup wipes a group's gameplay data back to a fresh pre-draft lobby:
// every player is fully overwritten with a blank Player{ID} (team/budget/
// tokens/principal all cleared — individual Set* calls can't null out
// TeamPrincipal, so this uses SavePlayer instead), and pilots/teams are
// re-seeded from the static templates (SaveTeam/SavePilot fully overwrite by
// id, so this cleanly undoes anything a draft pick assigned). It does NOT
// touch in-memory draft/setup/ready dispatcher state or the phase tracker —
// those live above the service layer; the HTTP handler clears them
// separately (see POST /groups/reset).
func (s *Service) ResetGroup(ctx context.Context, groupID int64) error {
	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, p := range players {
		if err := s.dynamic.SavePlayer(ctx, groupID, models.Player{ID: p.ID}); err != nil {
			return err
		}
	}
	return s.seedGroupWorld(ctx, groupID)
}
