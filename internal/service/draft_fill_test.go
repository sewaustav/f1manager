package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"

	"github.com/stretchr/testify/require"
)

func TestAutoFillByDescendingRating(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	// команда-игрок 100 (1 пилот уже есть) и бот 200 (пустой)
	r.SeedPlayer(g, models.Player{ID: 1, Team: 100})
	r.SeedTeam(g, models.Team{ID: 100})
	r.SeedTeam(g, models.Team{ID: 200})

	own := int64(1)
	gar := int64(100)
	r.SeedPilot(g, models.Pilot{ID: 500, Rating: 88, Team: &own, Garage: &gar}) // у команды 100 уже 1

	// свободные пилоты разного рейтинга
	r.SeedPilot(g, models.Pilot{ID: 501, Rating: 99})
	r.SeedPilot(g, models.Pilot{ID: 502, Rating: 70})
	r.SeedPilot(g, models.Pilot{ID: 503, Rating: 85})

	svc := New(r, r, nil, nil, nil)
	require.NoError(t, svc.AutoFillAfterDraft(ctx, g))

	// у обеих команд должно стать по 2 пилота
	t100, _ := r.GetPilotsByTeam(ctx, 100, g)
	t200, _ := r.GetPilotsByTeam(ctx, 200, g)
	require.Len(t, t100, 2)
	require.Len(t, t200, 2)

	// не осталось свободных
	free, _ := r.GetUnassignedPilots(ctx, g)
	require.Empty(t, free)

	// высший рейтинг (99) ушёл первым — команде 100 (ей нужен 1)
	ids100 := map[int64]bool{}
	for _, p := range t100 {
		ids100[p.ID] = true
	}
	require.True(t, ids100[501], "пилот 501 (99) должен попасть в первую заполняемую команду")
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
