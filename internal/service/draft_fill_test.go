package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"

	"github.com/stretchr/testify/require"
)

// Сценарий ревьюера: игрок берёт в свою команду (Ferrari, 100) двух пилотов,
// чьи дефолтные гаражи — команды-боты (Aston 200, Williams 201). Дефолтные пилоты
// Ferrari (Leclerc/Hamilton) остаются недобранными → освобождаются и перераспределяются
// по дефициту (в боты, потерявшие пилотов) по убыванию рейтинга.
func TestAutoFillReconciliation(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	r.SeedPlayer(g, models.Player{ID: 1, Team: 100})
	r.SeedTeam(g, models.Team{ID: 100}) // Ferrari (игрок)
	r.SeedTeam(g, models.Team{ID: 200}) // Aston (бот)
	r.SeedTeam(g, models.Team{ID: 201}) // Williams (бот)

	own := int64(1)
	g100, g200, g201 := int64(100), int64(200), int64(201)

	// дефолтные пилоты Ferrari — недобраны
	r.SeedPilot(g, models.Pilot{ID: 600, Rating: 94, Garage: &g100}) // Leclerc
	r.SeedPilot(g, models.Pilot{ID: 601, Rating: 95, Garage: &g100}) // Hamilton
	// Alonso (дефолт Aston) и Sainz (дефолт Williams) — забраны игроком в Ferrari
	r.SeedPilot(g, models.Pilot{ID: 602, Rating: 92, Team: &own, Garage: &g100}) // Alonso → 100
	r.SeedPilot(g, models.Pilot{ID: 603, Rating: 90, Team: &own, Garage: &g100}) // Sainz → 100
	// вторые пилоты ботов остаются на местах
	r.SeedPilot(g, models.Pilot{ID: 604, Rating: 70, Garage: &g200})
	r.SeedPilot(g, models.Pilot{ID: 605, Rating: 65, Garage: &g201})

	svc := New(r, r, nil, nil, nil)
	require.NoError(t, svc.AutoFillAfterDraft(ctx, g))

	// Ferrari укомплектована забранными пилотами
	t100, _ := r.GetPilotsByTeam(ctx, 100, g)
	require.Len(t, t100, 2)

	// все команды по 2 пилота, свободных не осталось
	t200, _ := r.GetPilotsByTeam(ctx, 200, g)
	t201, _ := r.GetPilotsByTeam(ctx, 201, g)
	require.Len(t, t200, 2)
	require.Len(t, t201, 2)
	free, _ := r.GetUnassignedPilots(ctx, g)
	require.Empty(t, free)

	// освобождённые Leclerc/Hamilton перераспределены по убыванию рейтинга:
	// первый по ID бот (200) получает высший рейтинг (601/Hamilton), затем 201 → 600.
	has := func(pilots []models.Pilot, id int64) bool {
		for _, p := range pilots {
			if p.ID == id {
				return true
			}
		}
		return false
	}
	require.True(t, has(t200, 601), "высший свободный рейтинг уходит первой команде с дефицитом")
	require.True(t, has(t201, 600))
}

func TestSwapBotPilotsOnlyBots(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	// команда-игрок 100, боты 200 и 300
	r.SeedPlayer(g, models.Player{ID: 1, Team: 100})
	r.SeedTeam(g, models.Team{ID: 100})
	r.SeedTeam(g, models.Team{ID: 200})
	r.SeedTeam(g, models.Team{ID: 300})

	g200 := int64(200)
	g300 := int64(300)
	g100 := int64(100)
	r.SeedPilot(g, models.Pilot{ID: 600, Garage: &g200})
	r.SeedPilot(g, models.Pilot{ID: 601, Garage: &g300})
	r.SeedPilot(g, models.Pilot{ID: 602, Garage: &g100})

	svc := New(r, r, nil, nil, nil)

	// обмен между ботами 200 и 300 — ок
	require.NoError(t, svc.SwapBotPilots(ctx, g, 200, 300, 600, 601))
	p600, _ := r.GetPilotByGroup(ctx, 600, g)
	p601, _ := r.GetPilotByGroup(ctx, 601, g)
	require.Equal(t, int64(300), *p600.Garage)
	require.Equal(t, int64(200), *p601.Garage)

	// обмен с командой игрока 100 — запрещён
	err := svc.SwapBotPilots(ctx, g, 200, 100, 600, 602)
	require.Error(t, err)
}
