package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	
	"f1/internal/engine"
	"f1/internal/models"
	"f1/internal/storage"
)

type CLI struct {
	store  *storage.Storage
	engine *engine.Engine
	reader *bufio.Reader
}

func NewCLI(store *storage.Storage, engine *engine.Engine) *CLI {
	return &CLI{
		store:  store,
		engine: engine,
		reader: bufio.NewReader(os.Stdin),
	}
}

func (c *CLI) Start() {
	fmt.Println("=== ДОБРО ПОЖАЛОВАТЬ В СИМУЛЯТОР ФОРМУЛЫ 1 ===")
	_ = c.store.ResetSession()
	
	fmt.Print("Введите количество игроков (человек): ")
	playersCountStr, _ := c.reader.ReadString('\n')
	playersCount, _ := strconv.Atoi(strings.TrimSpace(playersCountStr))
	
	players := make([]models.Player, playersCount)
	for i := 0; i < playersCount; i++ {
		fmt.Printf("Игрок %d, введите ваше имя: ", i+1)
		name, _ := c.reader.ReadString('\n')
		players[i].Name = strings.TrimSpace(name)
	}
	
	c.runDraft(players)
	c.fillBotTeams()
	c.configureSeason(players)
	c.runSimulation()
}

func (c *CLI) runDraft(players []models.Player) {
	fmt.Println("\n--- СТАРТ ДРАФТА КОМАНД И ПИЛОТОВ ---")
	teams, _ := c.store.GetTeams()
	pilots, _ := c.store.GetPilots()
	
	for i := range players {
		fmt.Printf("\n>>> Ход игрока %s <<<\n", players[i].Name)
		
		fmt.Println("Доступные команды:")
		for _, t := range teams {
			fmt.Printf("[%d] %s (Бюджет: %d млн)\n", t.ID, t.Name, t.Budget)
		}
		fmt.Print("Выберите ID команды: ")
		tIDStr, _ := c.reader.ReadString('\n')
		tID, _ := strconv.ParseInt(strings.TrimSpace(tIDStr), 10, 64)
		players[i].Team = tID
		
		// Драфт Пилота 1
		fmt.Println("Доступные пилоты:")
		for _, p := range pilots {
			if p.Team == "" || true { // Показываем всех для трансфера/выбора
				fmt.Printf("[%d] %s (Рейтинг: %d, Цена: %d млн, Текущая команда: %s)\n", p.ID, p.Name, p.Rating, p.Price, p.Team)
			}
		}
		fmt.Print("Выберите ID первого пилота: ")
		p1IDStr, _ := c.reader.ReadString('\n')
		p1ID, _ := strconv.ParseInt(strings.TrimSpace(p1IDStr), 10, 64)
		players[i].Pilot1 = p1ID
		
		// Простой трансферный интерфейс «на лету»
		c.store.ExecuteTransfer(p1ID, 0, tID, 0)
		
		fmt.Print("Выберите ID второго пилота: ")
		p2IDStr, _ := c.reader.ReadString('\n')
		p2ID, _ := strconv.ParseInt(strings.TrimSpace(p2IDStr), 10, 64)
		players[i].Pilot2 = p2ID
		c.store.ExecuteTransfer(p2ID, 0, tID, 0)
		
		c.store.SavePlayer(players[i])
	}
}

func (c *CLI) fillBotTeams() {
	fmt.Println("\n--- ЗАПОЛНЕНИЕ ПУСТЫХ СЛОТОВ БОТОВ РУКАМИ ---")
	pilots, _ := c.store.GetPilots()
	teams, _ := c.store.GetTeams()
	
	for _, t := range teams {
		// Ищем сколько пилотов числится за командой
		count := 0
		for _, p := range pilots {
			if p.Team == t.Name {
				count++
			}
		}
		
		for count < 2 {
			fmt.Printf("У команды %s не хватает пилота (всего %d/2). Введите ID свободного пилота для заполнения: ", t.Name, count)
			pIDStr, _ := c.reader.ReadString('\n')
			pID, _ := strconv.ParseInt(strings.TrimSpace(pIDStr), 10, 64)
			_ = c.store.ExecuteTransfer(pID, 0, t.ID, 0)
			count++
		}
	}
}

func (c *CLI) configureSeason(players []models.Player) {
	fmt.Println("\n--- РАСПРЕДЕЛЕНИЕ ТОКЕНОВ НА СЕЗОН ---")
	for _, p := range players {
		fmt.Printf("\nИгрок %s, распределите 120 токенов на болид.\n", p.Name)
		var car models.Car
		car.TeamID = p.Team
		
		fmt.Print("Токены на Аэродинамику: ")
		fmt.Scanln(&car.AeroDynamic)
		fmt.Print("Токены на Мотор: ")
		fmt.Scanln(&car.Engine)
		fmt.Print("Токены на Шасси: ")
		fmt.Scanln(&car.Chassis)
		fmt.Print("Токены на Днище: ")
		fmt.Scanln(&car.Floor)
		fmt.Print("Токены на Шины: ")
		fmt.Scanln(&car.Tyres)
		fmt.Print("Токены на Надежность (55 = 0% DNF): ")
		fmt.Scanln(&car.Reliability)
		
		_ = c.store.UpdateCar(car)
	}
}

func (c *CLI) runSimulation() {
	fmt.Println("\n=== СТАРТ СИМУЛЯЦИИ СЕЗОНА ===")
	tracks, _ := c.store.GetTracks()
	pilots, _ := c.store.GetPilots()
	teamsList, _ := c.store.GetTeams()
	
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
	
	c.engine.RecalculateRatings(driverStandings, teamStandings)
	
}
