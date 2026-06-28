package cli

import (
	"context"
	"f1/internal/models"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

func (c *CLI) rankMap(input map[int64]int) map[int64]int {
	type entry struct {
		key   int64
		value int
	}

	entries := make([]entry, 0, len(input))
	for k, v := range input {
		entries = append(entries, entry{k, v})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].value > entries[j].value
	})

	result := make(map[int64]int)
	rank := 1
	for i, e := range entries {
		if i > 0 && entries[i].value < entries[i-1].value {
			rank++
		}
		result[e.key] = rank
	}

	return result
}

func (c *CLI) resetTokensAndBudget(ctx context.Context, standing map[int64]int) {
	rank := c.rankMap(standing)
	
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
		tokens := 120 + 10 * (rank[pl.Team]-1)
		
		if err := c.store.UpdateTokens(ctx, pl.ID, tokens); err != nil {
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
