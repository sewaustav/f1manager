package cli

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
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


