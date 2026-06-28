package cli

import (
	"context"
	"f1/internal/engine"
	"f1/internal/models"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
)

func (c *CLI) сalculateUpdate(team models.Team, investment int, stage int64) *Updates {
	components := []int{team.BaseLevel, team.Engineer, team.SimLevel, team.TubeLevel}
	
	sum := 0
	minComp := components[0]
	maxComp := components[0]
	
	for _, val := range components {
		sum += val
		if val < minComp {
			minComp = val
		}
		if val > maxComp {
			maxComp = val
		}
	}
	
	avgBase := float64(sum) / float64(len(components))
	delta := float64(maxComp - minComp)
	
	minBonus := -5.0
	maxBonus := 3.0
	
	investmentModifier := ((float64(investment) / 15.0) * 3.0) - 1.5
	baseModifier := ((avgBase / 100.0) * 2.0) - 1.0
	deltaPenalty := (delta / 100.0) * 4.5
	
	if team.CarLevel > 95 {
		penaltyRatio := float64(team.CarLevel-95) / 5.0
		if penaltyRatio > 1.0 {
			penaltyRatio = 1.0
		}
		maxBonus = 3.0 - (penaltyRatio * 3.0)
	}
	
	randomValue := rand.Float64()
	rawBonus := minBonus + randomValue*(maxBonus-minBonus)
	
	finalRawBonus := rawBonus + investmentModifier + baseModifier - deltaPenalty
	
	if finalRawBonus > maxBonus {
		finalRawBonus = maxBonus
	}
	if finalRawBonus < minBonus {
		finalRawBonus = minBonus
	}
	
	roundedBonus := int(math.Round(finalRawBonus))
	
	return &Updates{
		Team:    team,
		Bonus:   roundedBonus + team.UpdateRating,
		Stage:   stage,
		Synergy: 0,
	}
}

func (c *CLI) updateCar(ctx context.Context, team models.Team, budget int, stage, playerID int64) *Updates {
	fmt.Print("Хотите обновить болид? (y/n): ")
	var ok string
	fmt.Scanln(&ok)
	if ok == "y" {
		fmt.Print("Что вы хотите обновить 1. Улучшить характеристики болида, 2. Адаптировать болид под стиль пилота : ")
		var choice string
		fmt.Scanln(&choice)
		if choice == "1" {
			fmt.Print("Введите сумму, которую хотите потратить(максимум 15 млн): ")
			var amount int
			fmt.Scanln(&amount)
			if amount > 15 {
				fmt.Println("Сумма не может быть больше 15 млн")
				return nil
			}
			if budget < amount {
				fmt.Println("Недостаточно средств")
				return nil
			}
			
			if err := c.store.UpdateBudget(ctx, playerID, amount); err != nil {
				fmt.Println("Ошибка при обновлении бюджета")
				return nil
			}
			
			return c.сalculateUpdate(team, amount, stage)
			
			
		} else if choice == "2" {
			fmt.Println("Выберите, сколько вы хотите потратить на адаптацию (2 пункта = 1 млн)")
			var amount int
			fmt.Scanln(&amount)
			if amount == 0 {
				return nil
			}
			synergy := amount * 2
			if amount < 0 {
				amount = amount * -1
			}
			if budget < amount {
				fmt.Println("Недостаточно средств")
				return nil
			}
			
			if err := c.store.UpdateBudget(ctx, playerID, amount); err != nil {
				fmt.Println("Ошибка при обновлении бюджета")
				return nil
			}
			
			return &Updates{
				Team: team,
				Synergy: synergy,
				Bonus: 0,
				Stage: stage,
			}
		} else {
			fmt.Println("Несуществующий выбор")
			return nil
		}
		
		
	}
	return nil
}

func (c *CLI) bringUpdates(ctx context.Context) {
	
}

func (c *CLI) runSimulation(ctx context.Context) {
	fmt.Println("\n=== СТАРТ СИМУЛЯЦИИ СЕЗОНА ===")
	tracks, err := c.store.GetTracks(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	
	driverPoints := make(map[int64]int)  // pilotID -> очки
	teamPoints   := make(map[int64]int)  // garageID -> очки
	
	driverStandings := make(map[string]int)
	teamStandings   := make(map[string]int)
	
	var updates []*Updates
	
	var snapshots []engine.PilotSeasonSnapshot
	var lastPilots []models.Pilot
	
	for _, track := range tracks {
		fmt.Printf("\n----------------------------------------\n")
		fmt.Printf("ЭТАП: %s\n", track.Name)
		fmt.Printf("----------------------------------------\n")
		
		pilots, err := c.store.GetActivePilots(ctx)
		if err != nil {
			fmt.Println("ошибка получения пилотов:", err)
			return
		}
		if len(pilots) < 20 {
			fmt.Println("Недостаточно пилотов для этапа!")
			return
		}
		lastPilots = pilots
		
		snapshots = make([]engine.PilotSeasonSnapshot, len(pilots))
		for i, p := range pilots {
			snapshots[i] = engine.PilotSeasonSnapshot{
				PilotID:         p.ID,
				BaseRating:      p.Rating,
				BaseQualiRating: p.QualifyingRating,
			}
		}
		
		teamsList, err := c.store.GetTeams(ctx)
		if err != nil {
			fmt.Println(err)
			return
		}
		
		players, err := c.store.GetPlayers(ctx)
		if err != nil {
			fmt.Println(err)
			return
		}
		
		var playersTeams []int64
		for _, p := range players {
			playersTeams = append(playersTeams, p.Team)
		}
		
		teams := make(map[int64]models.Team)
		cars := make(map[int64]models.Car)
		principals := make(map[int64]models.TeamPrincipal)
		
		for _, t := range teamsList {
			teams[t.ID] = t
			if slices.Contains(playersTeams, t.ID) {
				car, err := c.store.GetCar(ctx, t.ID)
				if err != nil {
					fmt.Println("error getting car", err)
					return
				}
				cars[t.ID] = car
				
				var principalID int64
				var playerID int64
				for _, p := range players {
					if p.Team == t.ID {
						principalID = *p.TeamPrincipal
						playerID = p.ID
					}
				}
				
				principal, err := c.store.GetTeamPrincipal(ctx, principalID)
				if err != nil {
					fmt.Println("principal", err)
					continue
				}
				principals[t.ID] = principal
				
				if track.ID == 3 || track.ID == 8 || track.ID == 13 {
					budget, err := c.store.GetBudget(ctx, playerID)
					if err != nil {
						fmt.Println("budget", err)
						continue
					}
					fmt.Println("budget", budget)
					var stage int
					switch track.ID {
					case 3: stage = 7
					case 8: stage = 12
					case 13: stage = 18
					}
					update := c.updateCar(ctx, t, budget, int64(stage), playerID)
					if update != nil {
						updates = append(updates, update)
						fmt.Println("update", update)
					}
				}
				
				
			} else {
				cars[t.ID] = models.Car{TeamID: t.ID, AeroDynamic: 20, Engine: 20, Chassis: 20, Floor: 20, Tyres: 20, Reliability: 20}
				principals[t.ID] = models.TeamPrincipal{Level: 20}
			}
		}
		
		if track.ID == 7 || track.ID == 12 || track.ID == 18 {
			fmt.Println("count updates",len(updates))
			for _, update := range updates {
				if update.Stage == track.ID {
					fmt.Println("make update", update)
					fmt.Println("syn", update.Team.CarSettings+update.Synergy)
					updatedTeam := models.Team{
						ID:           update.Team.ID,
						BaseLevel:    update.Team.BaseLevel,
						TubeLevel:    update.Team.TubeLevel,
						Engineer:     update.Team.Engineer,
						SimLevel:     update.Team.SimLevel,
						CarLevel:     update.Team.CarLevel+update.Bonus,
						CarSettings:  update.Team.CarSettings+update.Synergy,
					}
					if err := c.store.UpgradeTeam(ctx, updatedTeam); err != nil {
						fmt.Println("Ошибка обновления", err)
						continue
					}
				}
			}
		}
		
		results := c.engine.SimulateWeekend(ctx, track, pilots, teams, cars, principals, driverPoints, teamPoints)
		
		fmt.Printf("%-4s | %-20s | %-15s | %-5s | %-5s | %-6s\n", "Поз", "Пилот", "Команда", "Квала", "Гонка", "Очки")
		for _, res := range results {
			status := strconv.Itoa(res.RacePosition)
			if res.IsDNF {
				status = "DNF (" + res.DNFReason + ")"
			}
			fmt.Printf("%-4d | %-20s | %-15s | %-5d | %-5s | +%-5d\n", res.RacePosition, res.PilotName, res.TeamName, res.QualiPosition, status, res.Points)
			
			driverPoints[res.PilotID] += res.Points
			driverStandings[res.PilotName] += res.Points
			
			if res.GarageID != 0 {
				teamPoints[res.GarageID] += res.Points
				teamStandings[res.TeamName] += res.Points
			}
		}
	}
	
	// Вывод результатов сезона
	fmt.Println("\n========================================")
	fmt.Println("ИТОГОВЫЙ ЗАЧЕТ ПИЛОТОВ (WDC):")
	fmt.Println("========================================")
	for name, pts := range driverStandings {
		fmt.Printf("%-25s : %d очков\n", name, pts)
	}
	
	fmt.Println("\n========================================")
	fmt.Println("КУБОК КОНСТРУКТОРОВ (WCC):")
	fmt.Println("========================================")
	for name, pts := range teamStandings {
		fmt.Printf("%-25s : %d очков\n", name, pts)
	}
	
	// Передаем результаты, используя финальное состояние пилотов и snapshot'ов
	c.engine.UpdateAfterSeason(ctx, engine.SeasonStandings{
		DriverPoints: driverPoints,
		TeamPoints:   teamPoints,
		Pilots:       lastPilots,
		Snapshots:    snapshots,
	})
	
	c.resetTokensAndBudget(ctx, teamPoints)
	c.crossSeason(ctx)
}
