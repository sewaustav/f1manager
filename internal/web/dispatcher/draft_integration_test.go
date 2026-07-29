package dispatcher_test

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"
	"f1/internal/service"
	"f1/internal/web/dispatcher"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

// nopNotifier игнорирует уведомления — в интеграционном тесте проверяем состояние репозитория.
type nopNotifier struct{}

func (nopNotifier) SendUser(int64, []byte)       {}
func (nopNotifier) BroadcastGroup(int64, []byte) {}

func ice(v models.ICEName) *models.ICEName { return &v }

func TestFullDraftCycle(t *testing.T) {
	ctx := context.Background()
	const g = int64(1)
	r := memory.New()

	// 2 игрока
	r.SeedPlayer(g, models.Player{ID: 1})
	r.SeedPlayer(g, models.Player{ID: 2})

	// 2 клиентские команды под игроков + 1 бот
	r.SeedTeam(g, models.Team{ID: 100, Budget: 150, IsManufacturer: models.Client, ICE: models.Mercedes})
	r.SeedTeam(g, models.Team{ID: 101, Budget: 150, IsManufacturer: models.Client, ICE: models.Mercedes})
	r.SeedTeam(g, models.Team{ID: 200, Budget: 100, IsManufacturer: models.Client, ICE: models.Mercedes}) // бот

	// пилоты: 4 разбираются игроками (по 2), остальные — автозаполнение бота
	for _, p := range []models.Pilot{
		{ID: 1000, Rating: 95, Price: 20, Sponsors: 5},
		{ID: 1001, Rating: 94, Price: 20, Sponsors: 5},
		{ID: 1002, Rating: 93, Price: 20, Sponsors: 5},
		{ID: 1003, Rating: 92, Price: 20, Sponsors: 5},
		{ID: 1004, Rating: 80, Price: 10, Sponsors: 0},
		{ID: 1005, Rating: 70, Price: 10, Sponsors: 0},
	} {
		r.SeedPilot(g, p)
	}

	// принципалы
	r.SeedPrincipal(models.TeamPrincipal{ID: 5, Price: 10})
	r.SeedPrincipal(models.TeamPrincipal{ID: 6, Price: 10})

	r.SeedEngine(models.Engine{Engine: models.Mercedes, Price: 30})

	svc := service.New(r, r, nil, nil, nil)
	d := dispatcher.NewDraft(svc, nopNotifier{})
	d.SetShuffle(func([]int64) {}) // детерминизм: base = [1,2]

	require.NoError(t, d.StartDraft(ctx, g))

	// order = [1,2, 2,1, 1,2, 2,1]
	// Раунд 0: p1 команда 100, p2 команда 101
	// Раунд 1: p2 пилот 1001, p1 пилот 1000
	// Раунд 2: p1 пилот 1002, p2 пилот 1003
	// Раунд 3: p2 принципал 6, p1 принципал 5
	picks := []struct {
		user int64
		pick dto.Draft
	}{
		{1, dto.Draft{Pick: dto.DraftTeam, ItemID: 100, Engine: ice(models.Mercedes)}},
		{2, dto.Draft{Pick: dto.DraftTeam, ItemID: 101, Engine: ice(models.Mercedes)}},
		{2, dto.Draft{Pick: dto.DraftPilot, ItemID: 1001}},
		{1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1000}},
		{1, dto.Draft{Pick: dto.DraftPilot, ItemID: 1002}},
		{2, dto.Draft{Pick: dto.DraftPilot, ItemID: 1003}},
		{2, dto.Draft{Pick: dto.DraftPrincipal, ItemID: 6}},
		{1, dto.Draft{Pick: dto.DraftPrincipal, ItemID: 5}},
	}
	for _, p := range picks {
		require.NoError(t, d.SubmitPick(ctx, p.user, g, p.pick), "pick %+v", p)
	}

	// Ростеры валидны: у каждого игрока команда + 2 пилота + принципал
	for _, uid := range []int64{1, 2} {
		player, err := svc.ListGroupPlayers(ctx, g)
		require.NoError(t, err)
		require.Contains(t, player, uid)
	}

	p1, _ := r.GetPlayer(ctx, 1, g)
	require.Equal(t, int64(100), p1.Team)
	require.NotNil(t, p1.TeamPrincipal)
	pilots1, _ := r.GetPilotsByTeam(ctx, 100, g)
	require.Len(t, pilots1, 2)

	// Экономика p1: команда первым пиком (spent=0), engine client 40 → 150-0-40=110;
	// два пилота по 15 → 110-15-15=80; принципал 10 → 70.
	require.Equal(t, 70, p1.Budget)

	// Автозаполнение: бот 200 получил 2 свободных пилота (1004,1005)
	bot, _ := r.GetPilotsByTeam(ctx, 200, g)
	require.Len(t, bot, 2)
	free, _ := r.GetUnassignedPilots(ctx, g)
	require.Empty(t, free)
}
