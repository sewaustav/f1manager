package cli

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	
	"f1/internal/engine"
	"f1/internal/models"
	"f1/internal/storage"
)

type Updates struct {
	Team models.Team
	Bonus int
	Stage int64
	Synergy int
}

type CLI struct {
	store  storage.F1Repo
	engine *engine.Engine
	reader *bufio.Reader
}

func NewCLI(store storage.F1Repo, engine *engine.Engine) *CLI {
	return &CLI{
		store:  store,
		engine: engine,
		reader: bufio.NewReader(os.Stdin),
	}
}

func (c *CLI) Start(ctx context.Context) {
	
	
	fmt.Println("=== ДОБРО ПОЖАЛОВАТЬ В СИМУЛЯТОР ФОРМУЛЫ 1 ===")
	_ = c.store.ResetSession(ctx)
	
	fmt.Print("Введите количество игроков (человек): ")
	playersCountStr, _ := c.reader.ReadString('\n')
	playersCount, _ := strconv.Atoi(strings.TrimSpace(playersCountStr))
	
	players := make([]models.Player, playersCount)
	for i := 0; i < playersCount; i++ {
		fmt.Printf("Игрок %d, введите ваше имя: ", i+1)
		name, _ := c.reader.ReadString('\n')
		players[i].Name = strings.TrimSpace(name)
	}
	
	
	c.runDraft(ctx, players)
	c.fillBotTeams(ctx)
	c.configureSeason(ctx)
	c.runSimulation(ctx)
}

func (c *CLI) runDraft(ctx context.Context, players []models.Player) {
	
	fmt.Println("\n--- СТАРТ ДРАФТА КОМАНД И ПИЛОТОВ ---")
	teams, _ := c.store.CreateTeams(ctx)
	_ = c.store.CreatePilots(ctx)
	
	
	for i := range players {
		fmt.Printf("\n>>> Ход игрока %s <<<\n", players[i].Name)
		//fmt.Println(players[i])
		
		fmt.Println("Доступные команды:")
		for _, t := range teams {
			fmt.Printf("[%d] %s (Бюджет: %d млн)\n", t.ID, t.Name, t.Budget)
		}
		fmt.Print("Выберите ID команды: ")
		tIDStr, _ := c.reader.ReadString('\n')
		tID, _ := strconv.ParseInt(strings.TrimSpace(tIDStr), 10, 64)
		players[i].Team = tID
		
		fmt.Println(players[i])
		
		team, err := c.store.GetTeam(ctx, tID)
		if err != nil {
			fmt.Println("Ошибка при получении бюджета команды:", err)
			continue
		}
		
		budget := team.Budget
		
		player := models.Player{
			Team: tID,
			Name: players[i].Name,
			Budget: budget,
		}
		
		id, err := c.store.SavePlayer(ctx, player) 
		if err != nil {
			fmt.Println("Ошибка при сохранении игрока:", err)
			continue
		}
		
		// Драфт Пилота 1
		fmt.Println("Доступные пилоты:")
		pilots, err := c.store.GetPilots(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении пилотов:", err)
			continue
		}
		if len(pilots) == 0 {
			fmt.Println("Нет доступных пилотов для драфта.")
			continue
		}
		for _, p := range pilots {
			if p.Team == nil {
				fmt.Printf("[%d] %s (Рейтинг: %d, Цена: %d млн)\n", p.ID, p.Name, p.Rating, p.Price)
			}
		}
		fmt.Print("Выберите ID первого пилота: ")
		p1IDStr, _ := c.reader.ReadString('\n')
		p1ID, _ := strconv.ParseInt(strings.TrimSpace(p1IDStr), 10, 64)
		
		if pilots[p1ID-1].Sponsors != 0 {
			fmt.Println(pilots[p1ID-1].Sponsors)
			if err = c.store.UpdateBudget(ctx, id, pilots[p1ID-1].Sponsors*(-1)); err != nil {
				fmt.Println("Ошибка при обновлении бюджета:", err)
				continue
			}
			budget, err = c.store.GetBudget(ctx, id)
		}
		fmt.Println(pilots[p1ID-1].Price)
		if err = c.store.ExecuteTransfer(ctx, p1ID, 0, id, pilots[p1ID-1].Price); err != nil {
			fmt.Println("Ошибка при выполнении трансфера:", err)
			continue
		}
		
		fmt.Print("Выберите ID второго пилота: ")
		p2IDStr, _ := c.reader.ReadString('\n')
		p2ID, _ := strconv.ParseInt(strings.TrimSpace(p2IDStr), 10, 64)
		
		if pilots[p2ID-1].Sponsors != 0 {
			fmt.Println(pilots[p2ID-1].Sponsors)
			if err = c.store.UpdateBudget(ctx, id, pilots[p2ID-1].Sponsors*(-1)); err != nil {
				fmt.Println("Ошибка при обновлении бюджета:", err)
				continue
			}
			budget, err = c.store.GetBudget(ctx, id)
		}
		fmt.Println(pilots[p2ID-1].Price)
		if err = c.store.ExecuteTransfer(ctx, p2ID, 0, id, pilots[p2ID-1].Price); err != nil {
			fmt.Println("Ошибка при выполнении трансфера:", err)
			continue
		}
		
		principals, err := c.store.GetTeamPrincipals(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении Team Principals:", err)
			return
		}
		for _, p := range principals {
			fmt.Printf("[%d] %s (Цена: %d, Уровень: %d)\n", p.ID, p.Name, p.Price, p.Level)
		}
		fmt.Print("Выберете айди Team Principal: ")
		principalIDStr, _ := c.reader.ReadString('\n')
		principalID, _ := strconv.ParseInt(strings.TrimSpace(principalIDStr), 10, 64)
		fmt.Println(principals[principalID-1].Price)
		teamPrincipal, err := c.store.GetTeamPrincipal(ctx, principalID)
		if err != nil {
			fmt.Println("Ошибка при получении Team Principal:", err)
			continue
		}
		fmt.Println(teamPrincipal)
		players[i].TeamPrincipal = &principalID
		if err := c.store.TeamPrincipalTransfer(ctx, principalID, 0, id, teamPrincipal.Price); err != nil {
			fmt.Println("Ошибка при выполнении трансфера Team Principal:", err)
			continue
		}
		
		playerProfile, err := c.store.GetPlayer(ctx, id)
		if err != nil {
			fmt.Println("Ошибка при получении профиля игрока:", err)
			continue
		}
		fmt.Println(playerProfile)
		
		
		c.chooseEngine(ctx, models.Player{
			ID: id,
			Team: player.Team,
		}, team)
		
		tokens, err := c.store.GetTokens(ctx, id)
		if err != nil {
			fmt.Println("Ошибка при получении токенов:", err)
			continue
		}
		
		budget, err = c.store.GetBudget(ctx, id)
		if err != nil {
			fmt.Println("Ошибка при получении бюджета:", err)
			continue
		}
		
		fmt.Println("У вас", tokens, "токенов")
		fmt.Println("У вас", budget, "миллионов")
		
		//var tokensToBy int
		//fmt.Print("Выберете количество токенов для покупки(1 миллион = 1 токен): ")
		//fmt.Scanln(&tokensToBy)
		//if tokensToBy != 0 {
		//	c.buyTokens(ctx, models.Player{
		//		ID: id,
		//		Team: player.Team,
		//	}, tokensToBy, 0)
		//}
		
	}
}

func (c *CLI) chooseEngine(ctx context.Context, player models.Player, team models.Team) {
	engines, err := c.store.GetEngines(ctx)
	if err != nil {
		fmt.Println("failed to get engines:", err)
		return
	}
	if team.IsManufacturer == models.Manufacture {
		var price int
		for _, e := range engines {
			if e.Engine == team.ICE {
				price = e.Price
				break
			}
		}
		fmt.Printf("player.ID=%d\n", player.ID)
		fmt.Println(price)
		if err := c.store.UpdateBudget(ctx, player.ID, price); err != nil {
			fmt.Println("failed to update budget:", err)
		}
		return
	} else if team.IsManufacturer == models.Semi {
		fmt.Print("Хотите использовать текущую конфигурацию или стать клиентом?")
		var answ string
		fmt.Scan(&answ)
		if answ == "да" {
			var price int
			for _, e := range engines {
				if e.Engine == team.ICE {
					price = e.Price
					break
				}
			}
			if err := c.store.UpdateBudget(ctx, player.ID, price); err != nil {
				fmt.Println("Ошибка при установке бюджета:", err)
				return
			}
		} else {
			tx, err := c.store.Begin(ctx)
			if err != nil {
				fmt.Println("failed to begin transaction:", err)
				return
			}
			defer tx.Rollback()
			
			txRepo := c.store.WithTx(tx)
			for _, e := range engines {
				var engineName string
				switch e.Engine {
				case models.ICEName(0): engineName = "Ferrari"
				case models.ICEName(1): engineName = "Mercedes"
				case models.ICEName(2): engineName = "RBPT"
				case models.ICEName(3): engineName = "Honda"
				case models.ICEName(4): engineName = "Audi"
				case models.ICEName(5): engineName = "BMW"
				case models.ICEName(6): engineName = "Toyota"
				case models.ICEName(7): engineName = "Cadillac"
				case models.ICEName(8): engineName = "Renault"
				case models.ICEName(9): engineName = "Self"
				}
				fmt.Println(e.ID, engineName)
			}
			var engineId int
			fmt.Print("Выберете айди Engine: ")
			fmt.Scanln(&engineId)
			fmt.Println(engineId)
			if err := txRepo.UpdateTeam(ctx, models.Team{ID: player.Team, ICE: models.ICEName(engineId-1)}); err != nil {
				fmt.Println("failed to update budget1:", err)
			}
			
			if err := txRepo.UpdateBudget(ctx, player.ID, engines[engineId-1].Price+10); err != nil {
				fmt.Println("failed to update budget2:", err)
			}
			
			if err := tx.Commit(); err != nil {
				fmt.Println("failed to commit transaction:", err)
				return
			}
		}
		
	} else {
		tx, err := c.store.Begin(ctx)
		if err != nil {
			fmt.Println("failed to begin transaction:", err)
			return
		}
		defer tx.Rollback()
		
		txRepo := c.store.WithTx(tx)
		for _, e := range engines {
			var engineName string
			switch e.Engine {
			case models.ICEName(0): engineName = "Ferrari"
			case models.ICEName(1): engineName = "Mercedes"
			case models.ICEName(2): engineName = "RBPT"
			case models.ICEName(3): engineName = "Honda"
			case models.ICEName(4): engineName = "Audi"
			case models.ICEName(5): engineName = "BMW"
			case models.ICEName(6): engineName = "Toyota"
			case models.ICEName(7): engineName = "Cadillac"
			case models.ICEName(8): engineName = "Renault"
			case models.ICEName(9): engineName = "Self"
			}
			fmt.Println(e.ID, engineName)
		}
		var engineId int
		fmt.Print("Выберете айди Engine: ")
		fmt.Scanln(&engineId)
		if err := txRepo.UpdateTeam(ctx, models.Team{ID: player.Team, ICE: models.ICEName(engineId-1)}); err != nil {
			fmt.Println("failed to update budget1:", err)
		}
		
		if err := txRepo.UpdateBudget(ctx, player.ID, engines[engineId-1].Price+10); err != nil {
			fmt.Println("failed to update budget2:", err)
		}
		
		if err := tx.Commit(); err != nil {
			fmt.Println("failed to commit transaction:", err)
			return
		}
		
	}
}

func (c *CLI) buyTokens(ctx context.Context, player models.Player, tokensToBuy, attempt int) {
	currentBalance, err := c.store.GetBudget(ctx, player.ID)
	if err != nil {
		fmt.Println("failed to get budget:", err)
		return
	}
	
	fmt.Println("--------")
	fmt.Printf("Current balance: %d\n", currentBalance)
	fmt.Println("--------")
	
	if currentBalance < tokensToBuy {
		fmt.Println("not enough tokens")
		if attempt < 3 {
			attempt++
			c.buyTokens(ctx, player, tokensToBuy, attempt)
		} else {
			fmt.Println("failed to buy tokens")
		}
	}
	
	if err := c.store.UpdateBudget(ctx, player.ID, tokensToBuy); err != nil {
		fmt.Println("failed to update budget:", err)
	}
	
	if err := c.store.UpdateTokens(ctx, player.ID, tokensToBuy); err != nil {
		fmt.Println("failed to update tokens:", err)
	}
	
	return
}

func (c *CLI) fillBotTeams(ctx context.Context) {
	
	fmt.Println("\n--- ЗАПОЛНЕНИЕ ПУСТЫХ СЛОТОВ БОТОВ РУКАМИ ---")
	pilots, _ := c.store.GetPilots(ctx)
	//players, _ := c.store.GetPlayers(ctx)
	teams, _ := c.store.GetTeams(ctx)
	
	for _, t := range teams {
		// Ищем сколько пилотов числится за командой
		count := 0
		for _, p := range pilots {
			if p.Garage != nil && *p.Garage == t.ID {
				count++
			}
		}
		
		for count != 2 {
			if count < 2 {
				fmt.Printf("У команды %s не хватает пилота (всего %d/2). Введите ID свободного пилота для заполнения: ", t.Name, count)
				pIDStr, _ := c.reader.ReadString('\n')
				pID, _ := strconv.ParseInt(strings.TrimSpace(pIDStr), 10, 64)
				if err := c.store.ExecuteTransfer(ctx, pID, 0, t.ID, 0); err != nil {
					fmt.Println("Ошибка при выполнении трансфера:", err)
					continue
				}
				count++
			} else {
				fmt.Printf("У команды %s слишком много пилотов (всего %d/2). Введите ID пилота для удаления: ", t.Name, count)
				pIDStr, _ := c.reader.ReadString('\n')
				pID, _ := strconv.ParseInt(strings.TrimSpace(pIDStr), 10, 64)
				if err := c.store.ExecuteTransfer(ctx, pID, t.ID, 0, 0); err != nil {
					fmt.Println("Ошибка при выполнении трансфера:", err)
					continue
				}
				count--
			}
		}
	}
}

func (c *CLI) putTokens(attempt, tokens int) (int, int, int, int, int, int, models.SettingsAngle) {
	var aeroDynamic, engineTokens, chassis, floor, tyres, reliability, angle int
	fmt.Print("Токены на аэродинамику: ")
	fmt.Scanln(&aeroDynamic)
	fmt.Print("Токены на Мотор: ")
	fmt.Scanln(&engineTokens)
	fmt.Print("Токены на Шасси: ")
	fmt.Scanln(&chassis)
	fmt.Print("Токены на Днище: ")
	fmt.Scanln(&floor)
	fmt.Print("Токены на Шины: ")
	fmt.Scanln(&tyres)
	fmt.Print("Токены на Надежность (55 = 0% DNF): ")
	fmt.Scanln(&reliability)
	fmt.Print("Настройка баланса: ")
	fmt.Scanln(&angle)
	
	if aeroDynamic + engineTokens + chassis + floor + tyres + reliability > tokens {
		fmt.Println("Сумма токенов должна быть равна %d!", tokens)
		if attempt < 3 {
			attempt++
			return c.putTokens(attempt + 1, tokens)
		}
		
		return 20, 20, 20, 20, 20, 20, models.SettingsAngle(0)
		
	}
	
	return aeroDynamic, engineTokens, chassis, floor, tyres, reliability, models.SettingsAngle(angle)
	
}

func (c *CLI) configureSeason(ctx context.Context) {
	
	fmt.Println("\n--- РАСПРЕДЕЛЕНИЕ ТОКЕНОВ НА СЕЗОН ---")
	
	players, err := c.store.GetPlayers(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	
	fmt.Println(players)
	
	for _, p := range players {
		fmt.Println(p)
		player, err := c.store.GetPlayer(ctx, p.ID)
		if err != nil {
			fmt.Println(err)
			continue
		}
		
		fmt.Printf("\nИгрок %s, распределите %d токенов на болид.\n", player.Name, player.Tokens)
		var car models.Car
		car.TeamID = p.Team
		
		car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability, car.SettingsAngle = c.putTokens(0, player.Tokens)
		
		if err := c.store.UpdateCar(ctx, car); err != nil {
			fmt.Println(err)
		}
	}
}

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
				
				if track.ID == 7 || track.ID == 12 || track.ID == 18 {
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
			} else {
				cars[t.ID] = models.Car{TeamID: t.ID, AeroDynamic: 20, Engine: 20, Chassis: 20, Floor: 20, Tyres: 20, Reliability: 20}
				principals[t.ID] = models.TeamPrincipal{Level: 20}
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
	
	c.resetTokensAndBudget(ctx)
	c.crossSeason(ctx)
}

func (c *CLI) resetTokensAndBudget(ctx context.Context) {
	pilots, err := c.store.GetPilots(ctx)
	if err != nil {
		fmt.Println("Ошибка при получении списка пилотов:", err)
		return
	}
	
	players, err := c.store.GetPlayers(ctx)
	if err != nil {
		fmt.Println("Ошибка при получении списка игроков:", err)
		return
	}
	for _, pl := range players {
		budget := 0
		for _, p := range pilots {
			if p.Team != nil && *p.Team == pl.ID {
				budget = budget + p.Price - p.Sponsors
				fmt.Println(pl.ID, "budget", budget)
			}
		}
		if pl.TeamPrincipal != nil {
			principal, err := c.store.GetTeamPrincipal(ctx, *pl.TeamPrincipal)
			if err != nil {
				fmt.Println("Ошибка при получении руководителя команды:", err)
				return
			}
			budget = budget + principal.Price
			fmt.Println(pl.ID, "budget", budget)
		}	
		team, err := c.store.GetTeam(ctx, pl.Team)
		if err != nil {
			fmt.Println("Ошибка при получении команды:", err)
			return
		}
		ice, err := c.store.GetEngine(ctx, int64(team.ICE))
		if err != nil {
			fmt.Println("Ошибка при получении двигателя:", err)
			return
		}
		budget = budget + ice.Price
		fmt.Println(pl.ID, "budget", budget)
		fmt.Println(team.Budget-budget)
		if err := c.store.UpdateBudget(ctx, pl.ID, -1*(team.Budget-budget)); err != nil {
			fmt.Println("Ошибка при установке бюджета:", err)
			return
		}
		if err := c.store.ResetTokens(ctx, pl.ID); err != nil {
			fmt.Println("Ошибка при сбросе токенов:", err)
			return
		}
	}
}

func (c *CLI) crossSeason(ctx context.Context) {
	
	var newSeason string
	fmt.Print("Сезон завершен! Начать новый? (y/n): ")
	fmt.Scanln(&newSeason)
	if newSeason != "y" {
		return
	}
	
	for {
		pilots, err := c.store.GetPilots(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении списка пилотов:", err)
			return
		}
		players, err := c.store.GetPlayers(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении списка игроков:", err)
			return
		}
		principals, err := c.store.GetTeamPrincipals(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении списка руководителей команд:", err)
			return
		}
		
		engines, err := c.store.GetEngines(ctx)
		if err != nil {
			fmt.Println("Ошибка при получении списка двигателей:", err)
			return
		}
		
		for _, player := range players {
			fmt.Println("Игрок: ", player.Name, "Бюджет: ", player.Budget, "Остальное: ", player.TeamPrincipal)
		}
		for _, pilot := range pilots {
			fmt.Println("Пилот: ", pilot.Name, "Команда: ", pilot.Team)
		}
		
		for _, principal := range principals { 
			fmt.Println(principal.ID, "Глава команды: ", principal.Name)
		}
		
		for _, e := range engines {
			var engineName string
			switch e.Engine {
			case models.ICEName(0): engineName = "Ferrari"
			case models.ICEName(1): engineName = "Mercedes"
			case models.ICEName(2): engineName = "RBPT"
			case models.ICEName(3): engineName = "Honda"
			case models.ICEName(4): engineName = "Audi"
			case models.ICEName(5): engineName = "BMW"
			case models.ICEName(6): engineName = "Toyota"
			case models.ICEName(7): engineName = "Cadillac"
			case models.ICEName(8): engineName = "Renault"
			case models.ICEName(9): engineName = "Self"
			}
			fmt.Println(e.ID, "Двигатель: ", engineName, e.BaseLevel, e.Price)
		}
		
		fmt.Println("\n========================================")
		fmt.Println("Доступные команды: ")
		fmt.Println("1. Трансфер: transfer <your_id> <pilot_id> <amount>")
		fmt.Println("2. Увольнение пилота: fire <your_id> pilot/principal <pilot_id/principal_id>")
		fmt.Println("3. Поменять мотор: engine <your_id> <engine_id>")
		fmt.Println("4. Поменять главу: change_principal <your_id> <principal_id> <amount>")
		fmt.Println("5. Совершить обмен change <your_id> <your_opponent_id> <your_pilot_id> <pilot_id> <amount>(0 если баш на баш)")
		fmt.Println("6. Начать сезон: start")
		
		commandStr, _ := c.reader.ReadString('\n')
		command := strings.Fields(commandStr)
		if command[0] == "start" {
			err := c.buildCarForNextSeason(ctx)
			if err != nil {
				return 
			}
			
			c.fillBotTeams(ctx)
			c.configureSeason(ctx)
			c.runSimulation(ctx)
			return
		} else if command[0] == "transfer" {
			playerID, _ := strconv.ParseInt(command[1], 10, 64)
			pilotID, _ := strconv.ParseInt(command[2], 10, 64)
			amount, _ := strconv.Atoi(command[3])
			if err := c.transfer(ctx, playerID, pilotID, amount); err != nil {
				fmt.Println(err)
				continue
			}
		} else if command[0] == "fire" {
			playerID, _ := strconv.ParseInt(command[1], 10, 64)
			pilotID, _ := strconv.ParseInt(command[3], 10, 64)
			fmt.Println(pilotID)
			if err := c.store.Fire(ctx, playerID, pilotID, command[2]); err != nil {
				fmt.Println(err)
				continue
			}
		} else if command[0] == "engine" {
			playerID, _ := strconv.ParseInt(command[1], 10, 64)
			engineID, _ := strconv.ParseInt(command[2], 10, 64)
			player, err := c.store.GetPlayer(ctx, playerID)
			if err != nil {
				fmt.Println(err)
				continue
			}
			
			enginePrice := engines[engineID].Price
			team, err := c.store.GetTeam(ctx, player.Team)
			if err != nil {
				fmt.Println(err)
				continue
			}
			if team.IsManufacturer == models.Manufacture {
				fmt.Println("you are manufacturer")
				continue
			}
			var currentPrice int 
			for _, e := range engines {
				if e.Engine == team.ICE {
					currentPrice = e.Price
				}
			}
			
			if currentPrice < enginePrice {
				if player.Budget < enginePrice-currentPrice {
					fmt.Println("not enough funds")
					continue
				}
			}
			if err := c.store.UpdateTeam(ctx, models.Team{
				ID:  player.Team,
				ICE: models.ICEName(engineID),
			}); err != nil {
				fmt.Println(err)
				continue
			}
			
		} else if command[0] == "change" {
			playerID, _ := strconv.ParseInt(command[1], 10, 64)
			opponentID, _ := strconv.ParseInt(command[2], 10, 64)
			yourPilotID, _ := strconv.ParseInt(command[3], 10, 64)
			pilotID, _ := strconv.ParseInt(command[4], 10, 64)
			amount, _ := strconv.Atoi(command[5])
			
			seller, err := c.store.GetPlayer(ctx, playerID)
			if err != nil {
				fmt.Println(err)
				continue
			}
			buyer, err := c.store.GetPlayer(ctx, opponentID)
			if err != nil {
				fmt.Println(err)
				continue
			}
			if amount > 0 {
				if seller.Budget < amount {
					fmt.Println("not enough funds")
					continue
				}
			} else if amount < 0 {
				if buyer.Budget < -amount {
					fmt.Println("not enough funds")
					continue
				}
			}
			tx, err := c.store.Begin(ctx)
			txRepo := c.store.WithTx(tx)
			defer tx.Rollback()
			
			if err := txRepo.ChangePilotTeam(ctx, yourPilotID, opponentID); err != nil {
				fmt.Println(err)
				continue
			}
			
			if err := txRepo.ChangePilotTeam(ctx, pilotID, playerID); err != nil {
				fmt.Println(err)
				continue
			}
			
			if err := tx.Commit(); err != nil {
				fmt.Println(err)
				continue
			}
			
		} else if command[0] == "change_principal" {
			playerID, _ := strconv.ParseInt(command[1], 10, 64)
			principalID, _ := strconv.ParseInt(command[2], 10, 64)
			amount, _ := strconv.Atoi(command[3])
			player, err := c.store.GetPlayer(ctx, playerID)
			if err != nil {
				fmt.Println(err)
				continue
			}
			if player.TeamPrincipal != nil {
				fmt.Println("You have team principal - fire first")
			}
			
			isFree := false
			
			for _, pl := range players {
				if pl.TeamPrincipal != nil && *pl.TeamPrincipal == principalID {
					fmt.Print("Игрок", pl.Name, ", игрок с айди ", playerID, "сделал предложение в размере", amount, "Принять? (y/n) ")
					var confirm string
					fmt.Scanln(&confirm)
					if confirm == "y" {
						if err := c.store.TeamPrincipalTransfer(ctx, principalID, pl.ID, playerID, amount); err != nil {
							fmt.Println(err)
						}
					} else {
						fmt.Println("Отказ")
					}
				} else {
					isFree = true
				}
			}
			
			if isFree {
				if err := c.store.TeamPrincipalTransfer(ctx, principalID, 0, playerID, amount); err != nil {
					fmt.Println(err)
				}
			}
			
		}
		
		
	}
}

func (c *CLI) transfer(ctx context.Context, playerID, pilotID int64, amount int) error {
	pilot, err := c.store.GetPilot(ctx, pilotID)
	if err != nil {
		return err
	}
	if pilot.Team == nil || *pilot.Team == 0 {
		if pilot.Price-pilot.Sponsors > amount+5 {
			return fmt.Errorf("not enough funds")
		}
		if err := c.store.ExecuteTransfer(ctx, pilotID, 0, playerID, amount); err != nil {
			return err
		}
		return nil
	}
	
	var confirm string 
	fmt.Println("Игрок ", pilot.Team, "Вы принимаете предложение от игрока ", playerID, "за пилота ", pilot.Name, "в размере ", amount, "? (y/n)")
	fmt.Scanln(&confirm)
	if confirm != "y" {
		return fmt.Errorf("user declined")
	}
	if err := c.store.ExecuteTransfer(ctx, pilotID, *pilot.Team, playerID, amount); err != nil {
		return err
	}
	
	return nil
}

func (c *CLI) diminishingReturn(x float64) float64 {
	const coefficient = 3.162277
	
	return coefficient * math.Sqrt(x)
}

func calcBonus(input int) int {
	weights := map[int][]int{
		0:  {45, 25, 15, 8, 4, 2, 1, 0, 0, 0, 0},
		1:  {20, 35, 20, 12, 6, 4, 2, 1, 0, 0, 0},
		2:  {10, 20, 35, 18, 9, 5, 2, 1, 0, 0, 0},
		3:  {5, 12, 18, 35, 15, 8, 4, 2, 1, 0, 0},
		4:  {2, 6, 10, 17, 35, 16, 8, 4, 1, 1, 0},
		5:  {1, 3, 6, 10, 15, 35, 16, 8, 4, 1, 1},
		6:  {0, 1, 3, 6, 10, 15, 35, 16, 8, 4, 2},
		7:  {0, 0, 1, 3, 6, 10, 15, 35, 16, 10, 4},
		8:  {0, 0, 0, 1, 3, 6, 10, 15, 35, 20, 10},
		9:  {0, 0, 0, 0, 1, 3, 6, 10, 15, 45, 20},
		10: {0, 0, 0, 0, 0, 5, 10, 15, 20, 45, 5},
	}
	
	currentWeights, exists := weights[input]
	if !exists {
		return input
	}
	
	roll := rand.Intn(100)
	accumulator := 0
	
	for choice, weight := range currentWeights {
		accumulator += weight
		if roll < accumulator {
			return choice
		}
	}
	
	return input
}

func (c *CLI) buildCarForNextSeason(ctx context.Context) error {
	players, err := c.store.GetPlayers(ctx)
	if err != nil {
		fmt.Println("error getting players", err)
		return err
	}
	
	for _, p := range players {
		fmt.Println(p)
		
		team, err := c.store.GetTeam(ctx, p.Team)
		if err != nil {
			fmt.Println("error getting team", err)
			return err
		}
		
		fmt.Println(p.Name,", сколько вы хотите вложить в болид?")
		
		fmt.Print("Улучшение базы(макс 10млн) :")
		var base int
		fmt.Scanln(&base)
		if base > 10 {
			fmt.Println("not enough funds")
			continue
		}
		fmt.Print("Улучшение трубы(макс 5млн) :")
		var tube int
		fmt.Scanln(&tube)
		if tube > 5 {
			fmt.Println("not enough funds")
			continue
		}
		
		var engineer int
		if team.Engineer < 95 {
			fmt.Print("Нанять инженеров(макс 5 млн) :")
			
			fmt.Scanln(&engineer)
			if engineer > 5 {
				fmt.Println("not enough funds")
				continue
			}
		} else {
			engineer = 0
		}
		
		fmt.Print("Улучшение симулятора(макс 5млн) :")
		var sim int
		fmt.Scanln(&sim)
		if sim > 5 {
			fmt.Println("not enough funds")
			continue
		}
		
		team, err = c.store.GetTeam(ctx, p.Team)
		if err != nil {
			fmt.Println("error getting team", err)
			return err
		}
		
		tx, err := c.store.Begin(ctx)
		if err != nil {
			fmt.Println("error starting transaction", err)
			return err
		}
		defer tx.Rollback()
		txRepo := c.store.WithTx(tx)
		
		newCarLvl := (team.BaseLevel + team.TubeLevel + team.Engineer + team.SimLevel + team.CarLevel) / 5
		
		if err = txRepo.UpgradeTeam(ctx, models.Team{
			ID:           team.ID,
			BaseLevel:    team.BaseLevel + calcBonus(base),
			TubeLevel:    team.TubeLevel + calcBonus(tube),
			Engineer:     team.Engineer + calcBonus(engineer),
			SimLevel:     team.SimLevel + calcBonus(sim),
			CarLevel:     newCarLvl,
			CarSettings:  team.CarSettings,
		}); err != nil {
			fmt.Println("error upgrading team", err)
			return err
		}
		
		
		if err := txRepo.UpdateBudget(ctx, p.ID, base); err != nil {
			fmt.Println("error updating budget", err)
		}
		if err := txRepo.UpdateBudget(ctx, p.ID, tube); err != nil {
			fmt.Println("error updating budget", err)
		}
		if err := txRepo.UpdateBudget(ctx, p.ID, engineer); err != nil {
			fmt.Println("error updating budget", err)
		}
		if err := txRepo.UpdateBudget(ctx, p.ID, sim); err != nil {
			fmt.Println("error updating budget", err)
		}
		
		if err := tx.Commit(); err != nil {
			fmt.Println("error committing transaction", err)
			return err
		}
	}
	return nil
}
