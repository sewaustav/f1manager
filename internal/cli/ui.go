package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	
	"f1/internal/engine"
	"f1/internal/models"
	"f1/internal/storage"
)

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
	c.configureSeason(ctx, players)
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
		
		//fmt.Println(players[i])
		
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
		
		if pilots[p1ID].Sponsors != 0 {
			if err = c.store.UpdateBudget(ctx, id, pilots[p1ID].Sponsors); err != nil {
				fmt.Println("Ошибка при обновлении бюджета:", err)
				continue
			}
			budget, err = c.store.GetBudget(ctx, id)
		}
		if err = c.store.ExecuteTransfer(ctx, p1ID, 0, id, pilots[p1ID].Price); err != nil {
			fmt.Println("Ошибка при выполнении трансфера:", err)
			continue
		}
		
		fmt.Print("Выберите ID второго пилота: ")
		p2IDStr, _ := c.reader.ReadString('\n')
		p2ID, _ := strconv.ParseInt(strings.TrimSpace(p2IDStr), 10, 64)
		
		if pilots[p2ID].Sponsors != 0 {
			if err = c.store.UpdateBudget(ctx, id, pilots[p2ID].Sponsors); err != nil {
				fmt.Println("Ошибка при обновлении бюджета:", err)
				continue
			}
			budget, err = c.store.GetBudget(ctx, id)
		}
		if err = c.store.ExecuteTransfer(ctx, p2ID, 0, id, pilots[p2ID].Price); err != nil {
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
		players[i].TeamPrincipal = principalID
		if err := c.store.TeamPrincipalTransfer(ctx, principalID, 0, id, principals[principalID].Price); err != nil {
			fmt.Println("Ошибка при выполнении трансфера Team Principal:", err)
			continue
		}
		
		playerProfile, err := c.store.GetPlayer(ctx, id)
		if err != nil {
			fmt.Println("Ошибка при получении профиля игрока:", err)
			continue
		}
		fmt.Println(playerProfile)
		
	}
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
		
		for count < 2 {
			fmt.Printf("У команды %s не хватает пилота (всего %d/2). Введите ID свободного пилота для заполнения: ", t.Name, count)
			pIDStr, _ := c.reader.ReadString('\n')
			pID, _ := strconv.ParseInt(strings.TrimSpace(pIDStr), 10, 64)
			if err := c.store.ExecuteTransfer(ctx, pID, 0, t.ID, 0); err != nil {
				fmt.Println("Ошибка при выполнении трансфера:", err)
				continue
			}
			count++
		}
	}
}

func (c *CLI) putTokens() (int, int, int, int, int, int) {
	var aeroDynamic, engine, chassis, floor, tyres, reliability int
	fmt.Print("Токены на аэродинамику")
	fmt.Scanln(&aeroDynamic)
	fmt.Print("Токены на Мотор: ")
	fmt.Scanln(&engine)
	fmt.Print("Токены на Шасси: ")
	fmt.Scanln(&chassis)
	fmt.Print("Токены на Днище: ")
	fmt.Scanln(&floor)
	fmt.Print("Токены на Шины: ")
	fmt.Scanln(&tyres)
	fmt.Print("Токены на Надежность (55 = 0% DNF): ")
	fmt.Scanln(&reliability)
	return aeroDynamic, engine, chassis, floor, tyres, reliability
	
}

func (c *CLI) configureSeason(ctx context.Context, players []models.Player) {
	
	fmt.Println("\n--- РАСПРЕДЕЛЕНИЕ ТОКЕНОВ НА СЕЗОН ---")
	for _, p := range players {
		fmt.Printf("\nИгрок %s, распределите 120 токенов на болид.\n", p.Name)
		var car models.Car
		car.TeamID = p.Team
		
		car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability = c.putTokens()
		
		if car.Reliability + car.AeroDynamic + car.Engine + car.Chassis + car.Floor + car.Tyres > 120 {
			fmt.Println("Сумма токенов должна быть равна 120!")
			
		}
		
		_ = c.store.UpdateCar(ctx, car)
	}
}

func (c *CLI) runSimulation(ctx context.Context) {
	
	fmt.Println("\n=== СТАРТ СИМУЛЯЦИИ СЕЗОНА ===")
	tracks, _ := c.store.GetTracks(ctx)
	pilots, _ := c.store.GetActivePilots(ctx)
	teamsList, _ := c.store.GetTeams(ctx)
	
	if len(pilots) < 20 {
		fmt.Println("Недостаточно пилотов для начала сезона!")
		for _, p := range pilots {
			fmt.Println(p)
		}
		return
	}
	
	teams := make(map[int64]models.Team)
	cars := make(map[int64]models.Car)
	principals := make(map[int64]models.TeamPrincipal)
	
	for _, t := range teamsList {
		teams[t.ID] = t
		cars[t.ID] = models.Car{TeamID: t.ID, AeroDynamic: 20, Engine: 20, Chassis: 20, Floor: 20, Tyres: 20, Reliability: 20}
		principals[t.ID] = models.TeamPrincipal{Level: 20} // Усредненный дефолт шефа
	}
	
	driverStandings := make(map[string]int)
	teamStandings := make(map[string]int)
	
	for _, track := range tracks {
		fmt.Printf("\n----------------------------------------\n")
		fmt.Printf("ЭТАП: %s\n", track.Name)
		fmt.Printf("----------------------------------------\n")
		
		results := c.engine.SimulateWeekend(track, pilots, teams, cars, principals)
		
		fmt.Printf("%-4s | %-20s | %-15s | %-5s | %-5s | %-6s\n", "Поз", "Пилот", "Команда", "Квала", "Гонка", "Очки")
		for _, res := range results {
			status := strconv.Itoa(res.RacePosition)
			if res.IsDNF {
				status = "DNF (" + res.DNFReason + ")"
			}
			fmt.Printf("%-4d | %-20s | %-15s | %-5d | %-5s | +%-5d\n", res.RacePosition, res.PilotName, res.TeamName, res.QualiPosition, status, res.Points)
			
			driverStandings[res.PilotName] += res.Points
			teamStandings[res.TeamName] += res.Points
		}
	}
	
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
	
	//c.engine.RecalculateRatings(driverStandings, teamStandings)
	
}
