package service

import (
	"context"
	"errors"
	"f1/internal/engine"
	"f1/internal/models"
	repo "f1/internal/new_storage"
	"f1/internal/web/dto"
	"fmt"
	"math"
	"slices"
)

// transferConfirmMsg — WS-сообщение, которое уходит владельцу пилота.
type transferConfirmMsg struct {
	Type    string `json:"type"`
	PilotID int64  `json:"pilot_id"`
	Price   int    `json:"price"`
}


type Service struct {
	static          repo.StaticRepo
	dynamic         repo.DynamicRepo
	engine          *engine.Engine
	updateCache     UpdateCache
	sessionProvider  SessionProvider
}

func New(static repo.StaticRepo, dynamic repo.DynamicRepo, eng *engine.Engine, updateCache UpdateCache, sessionProvider SessionProvider) *Service {
	return &Service{
		static:          static,
		dynamic:         dynamic,
		engine:          eng,
		updateCache:     updateCache,
		sessionProvider: sessionProvider,
	}
}

func (s *Service) Simulate(ctx context.Context, groupID, stage int64) ([]models.RaceResult, error) {
	if stage == 7 || stage == 12 || stage == 18 {
		s.bringUpdate(ctx, groupID, stage)
	}

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

	if err = s.dynamic.UpdateBudget(ctx, userID, groupID, int(math.Abs(float64(req.Coast)))); err != nil {
		return err
	}

	switch req.Type {
	case dto.CarUpdate:
		update := s.calculateUpdate(team, req.Coast, req.Stage)
		newUpdate := Update{
			Key:      fmt.Sprintf("%d-%d", userID, groupID),
			PlayerID: userID,
			GroupID:  groupID,
			TeamID:   player.Team,
			Stage:    req.Stage,
			Bonus:    update.Bonus,
			Type:     Car,
		}
		return s.updateCache.PutUpdate(ctx, newUpdate)

	case dto.SynergyUpdate:
		return s.updateCache.PutUpdate(ctx, Update{
			Key:      fmt.Sprintf("%d-%d", userID, groupID),
			PlayerID: userID,
			GroupID:  groupID,
			TeamID:   player.Team,
			Stage:    req.Stage,
			Bonus:    req.Coast,
			Type:     Synergy,
		})

	default:
		return errors.New("неизвестный тип обновления")
	}
}

// ChooseSetup — игрок распределяет токены на настройки болида перед гонкой.
// Токены раздаются заново перед каждой гонкой, поэтому баланс НЕ уменьшается
// после сетапа — распределение лишь применяется к болиду.
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

	// Токены не списываем: баланс сохраняется до следующей пред-гоночной раздачи.
	return s.dynamic.UpdateCar(ctx, player.Team, groupID, car)
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
	if req.Base > 10 {
		req.Base = 10
	}
	if req.Engineer > 5 {
		req.Engineer = 5
	}
	if req.Tube > 5 {
		req.Tube = 5
	}
	if req.Sim > 5 {
		req.Sim = 5
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
	
	// for future - include drivers to the function
	newCar := (team.CarLevel + team.BaseLevel + team.Engineer + team.TubeLevel + team.SimLevel) / 5
	
	newBase := min(team.BaseLevel + s.getBonus(req.Base, 10), 100)
	newEngineer := min(team.Engineer + s.getBonus(req.Engineer, 5), 5)
	newTube := min(team.TubeLevel + s.getBonus(req.Tube, 5), 5)
	newSim := min(team.SimLevel + s.getBonus(req.Sim, 5), 5)
	
	updatedTeam.BaseLevel = newBase
	updatedTeam.Engineer = newEngineer
	updatedTeam.TubeLevel = newTube
	updatedTeam.SimLevel = newSim
	updatedTeam.CarLevel = newCar

	if err = s.dynamic.UpdateTeam(ctx, userID, updatedTeam); err != nil {
		return err
	}

	return s.dynamic.UpdateBudget(ctx, userID, groupID, total)
}


// getOwnerByTeam возвращает userID игрока, которому принадлежит данная команда в группе.
func (s *Service) getOwnerByTeam(ctx context.Context, teamID, groupID int64) (int64, error) {
	players, err := s.dynamic.GetPlayers(ctx, groupID)
	if err != nil {
		return 0, err
	}
	for _, p := range players {
		if p.Team == teamID {
			return p.ID, nil
		}
	}
	return 0, errors.New("no player owns this team")
}

// PrincipalTransfer — смена тимпринципала.
// TODO - FIX we need to check is principal free
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
// TODO - FIX reset tockens by team place
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
