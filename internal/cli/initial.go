package cli

import (
	"context"
	"f1/internal/models"
	"fmt"
	"strconv"
	"strings"
)

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
