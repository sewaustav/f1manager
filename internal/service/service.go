package service

import (
	"context"
	"errors"
	"f1/internal/engine"
	"f1/internal/models"
	repo "f1/internal/new_storage"
	"f1/internal/web/dto"
	"fmt"
	"slices"
)

type Service struct {
	static  repo.StaticRepo
	dynamic repo.DynamicRepo
	engine  *engine.Engine
}

func New(static repo.StaticRepo, dynamic repo.DynamicRepo, eng *engine.Engine) *Service {
	return &Service{
		static:  static,
		dynamic: dynamic,
		engine:  eng,
	}
}

func (s *Service) Simulate(ctx context.Context, groupID, stage int64) ([]models.RaceResult, error) {
	track, err := s.static.GetTrack(ctx, stage)
	if err != nil {
		return nil, err
	}

	pilots, err := s.dynamic.GetPilotsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	teamList, err := s.dynamic.GetTeamsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return nil, err
	}

	var playersTeams []int64
	for _, p := range players {
		playersTeams = append(playersTeams, p.Team)
	}

	teams := make(map[int64]models.Team)
	cars := make(map[int64]models.Car)
	principals := make(map[int64]models.TeamPrincipal)

	for _, t := range teamList {
		teams[t.ID] = t
		if slices.Contains(playersTeams, t.ID) {
			car, err := s.dynamic.GetCar(ctx, t.ID, groupID)
			if err != nil {
				fmt.Println("error getting car", err)
				return nil, err
			}
			cars[t.ID] = car

			var principalID int64
			for _, p := range players {
				if p.Team == t.ID {
					principalID = *p.TeamPrincipal
				}
			}

			principal, err := s.static.GetTeamPrincipal(ctx, principalID)
			if err != nil {
				fmt.Println("principal", err)
				continue
			}
			principals[t.ID] = principal
		} else {
			cars[t.ID] = models.Car{TeamID: t.ID, AeroDynamic: 20, Engine: 20, Chassis: 20, Floor: 20, Tyres: 20, Reliability: 20}
			principals[t.ID] = models.TeamPrincipal{Level: 20}
		}
	}

	var driverPoints, teamPoints map[int64]int
	if stage == 1 {
		driverPoints = make(map[int64]int)
		teamPoints = make(map[int64]int)
	} else {
		driverPoints, teamPoints, err = s.GetStanding(ctx, groupID)
		if err != nil {
			return nil, err
		}
	}

	results := s.engine.SimulateWeekend(ctx, track, pilots, teams, cars, principals, driverPoints, teamPoints)

	if err = s.dynamic.HandleRace(ctx, results, groupID); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *Service) GetStanding(ctx context.Context, groupID int64) (map[int64]int, map[int64]int, error) {
	return s.dynamic.GetStanding(ctx, groupID)
}

// MakeUpdate — обновление болида за бюджет между этапами (car upgrade или synergy).
func (s *Service) MakeUpdate(ctx context.Context, userID int64, req dto.Updates) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}

	if req.Coast > 15 {
		return errors.New("максимальная сумма обновления — 15 млн")
	}

	budget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if budget < req.Coast {
		return errors.New("недостаточно бюджета")
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	team, err := s.dynamic.GetTeamByGroup(ctx, player.Team, groupID)
	if err != nil {
		return err
	}

	if err = s.dynamic.UpdateBudget(ctx, userID, groupID, req.Coast); err != nil {
		return err
	}

	switch req.Type {
	case dto.CarUpdate:
		update := s.сalculateUpdate(team, req.Coast, req.Stage)
		updatedTeam := team
		updatedTeam.CarLevel = team.CarLevel + update.Bonus
		return s.dynamic.UpgradeTeam(ctx, groupID, updatedTeam)

	case dto.SynergyUpdate:
		synergy := req.Coast * 2
		updatedTeam := team
		updatedTeam.CarSettings = team.CarSettings + synergy
		return s.dynamic.UpgradeTeam(ctx, groupID, updatedTeam)

	default:
		return errors.New("неизвестный тип обновления")
	}
}

// ChooseSetup — игрок выбирает настройки болида перед гонкой (токены).
// Вызывается диспетчером; сама логика применения — в SetupDispatcher.
func (s *Service) ChooseSetup(ctx context.Context, userID int64, setup dto.Setup) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}

	tokens, err := s.dynamic.GetTokens(ctx, userID, groupID)
	if err != nil {
		return err
	}

	total := setup.AeroDynamic + setup.Engine + setup.Chassis + setup.Floor + setup.Tyres + setup.Reliability
	if total > tokens {
		return fmt.Errorf("недостаточно токенов: нужно %d, есть %d", total, tokens)
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	car := models.Car{
		TeamID:        player.Team,
		AeroDynamic:   setup.AeroDynamic,
		Engine:        setup.Engine,
		Chassis:       setup.Chassis,
		Floor:         setup.Floor,
		Tyres:         setup.Tyres,
		Reliability:   setup.Reliability,
		SettingsAngle: setup.SettingsAngle,
	}

	if err = s.dynamic.UpdateCar(ctx, player.Team, groupID, car); err != nil {
		return err
	}

	remaining := tokens - total
	return s.dynamic.UpdateTokens(ctx, userID, groupID, remaining)
}

// MakeTokenSetup — применить настройки токенов на болид (cross-season, перед новым сезоном).
func (s *Service) MakeTokenSetup(ctx context.Context, userID int64, setup dto.Setup) error {
	return s.ChooseSetup(ctx, userID, setup)
}

// UpdateBase — распределить бюджет в инфраструктуру (база, инженер, аэротруба, симулятор).
func (s *Service) UpdateBase(ctx context.Context, userID int64, req dto.BaseUpdate) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}

	total := req.Base + req.Engineer + req.Tube + req.Sim
	if total == 0 {
		return errors.New("нужно указать хотя бы одно значение")
	}

	budget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if budget < total {
		return fmt.Errorf("недостаточно бюджета: нужно %d млн, есть %d млн", total, budget)
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	team, err := s.dynamic.GetTeamByGroup(ctx, player.Team, groupID)
	if err != nil {
		return err
	}

	updatedTeam := team
	updatedTeam.BaseLevel += req.Base
	updatedTeam.Engineer += req.Engineer
	updatedTeam.TubeLevel += req.Tube
	updatedTeam.SimLevel += req.Sim

	if err = s.dynamic.UpdateTeam(ctx, userID, updatedTeam); err != nil {
		return err
	}

	return s.dynamic.UpdateBudget(ctx, userID, groupID, total)
}

// PilotTransfer — покупка пилота у другого игрока или свободного агента.
func (s *Service) PilotTransfer(ctx context.Context, userID int64, req dto.PilotTransfer) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}

	budget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}

	pilot, err := s.static.GetPilot(ctx, req.PilotID)
	if err != nil {
		return err
	}

	if budget < pilot.Price {
		return fmt.Errorf("недостаточно бюджета: нужно %d млн, есть %d млн", pilot.Price, budget)
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	var fromTeamID int64
	if pilot.Team != nil {
		fromTeamID = *pilot.Team
	}

	if err = s.dynamic.ExecutePilotTransfer(ctx, pilot.ID, fromTeamID, player.Team, pilot.Price); err != nil {
		return err
	}

	return s.dynamic.UpdateBudget(ctx, userID, groupID, pilot.Price)
}

// PrincipalTransfer — смена тимпринципала.
func (s *Service) PrincipalTransfer(ctx context.Context, userID int64, req dto.PrincipalTransfer) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}

	budget, err := s.dynamic.GetBudget(ctx, userID, groupID)
	if err != nil {
		return err
	}

	principal, err := s.static.GetTeamPrincipal(ctx, req.PrincipalID)
	if err != nil {
		return err
	}

	if budget < principal.Price {
		return fmt.Errorf("недостаточно бюджета: нужно %d млн, есть %d млн", principal.Price, budget)
	}

	player, err := s.dynamic.GetPlayer(ctx, userID, groupID)
	if err != nil {
		return err
	}

	if err = s.dynamic.ExecutePrincipalTransfer(ctx, principal.ID, principal.TeamID, player.Team, principal.Price); err != nil {
		return err
	}

	return s.dynamic.UpdateBudget(ctx, userID, groupID, principal.Price)
}

// ResetSeason — сброс после сезона (токены/бюджет).
func (s *Service) ResetSeason(ctx context.Context, groupID int64) error {
	return s.dynamic.ResetTokensAndBudget(ctx, groupID)
}

// getUserGroup — вспомогательный метод получения группы пользователя.
func (s *Service) getUserGroup(ctx context.Context, userID int64) (int64, error) {
	groupID, err := s.dynamic.GetUserGroup(ctx, userID)
	if err != nil {
		return 0, err
	}
	if groupID == nil {
		return 0, errors.New("пользователь не состоит в группе")
	}
	return *groupID, nil
}

func (s *Service) GetLastRaceResults(ctx context.Context, groupID int64) ([]models.RaceResult, int64, error) {
	return s.dynamic.GetLastRaceResults(ctx, groupID)
}

// PickItem — заглушка для будущего драфта (реализация через веб-хуки).
func (s *Service) PickItem(ctx context.Context, userID int64, item dto.DraftItem) error {
	return nil
}
