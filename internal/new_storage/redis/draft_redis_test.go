package redis_test

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"
	redisrepo "f1/internal/new_storage/redis"
	"f1/internal/service"
	"f1/internal/web/dto"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func ice(v models.ICEName) *models.ICEName { return &v }

// Прогоняем полный цикл драфта против настоящего Redis (miniredis):
// динамика — в Redis, статика (моторы/принципалы) — в памяти.
func TestDraftOnRedis(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	dyn := redisrepo.NewDynamic(rdb)

	const g = int64(1)
	// сид динамики в Redis
	require.NoError(t, dyn.SavePlayer(ctx, g, models.Player{ID: 1}))
	require.NoError(t, dyn.SaveTeam(ctx, g, models.Team{ID: 100, Budget: 150, IsManufacturer: models.Client, ICE: models.Mercedes}))
	require.NoError(t, dyn.SaveTeam(ctx, g, models.Team{ID: 200, Budget: 100, IsManufacturer: models.Client, ICE: models.Mercedes})) // бот
	// пилоты игрока
	require.NoError(t, dyn.SavePilot(ctx, g, models.Pilot{ID: 1000, Rating: 95, Price: 20, Sponsors: 5}))
	require.NoError(t, dyn.SavePilot(ctx, g, models.Pilot{ID: 1001, Rating: 90, Price: 20, Sponsors: 5}))
	// свободные для автозаполнения бота
	require.NoError(t, dyn.SavePilot(ctx, g, models.Pilot{ID: 1002, Rating: 80, Price: 10}))
	require.NoError(t, dyn.SavePilot(ctx, g, models.Pilot{ID: 1003, Rating: 70, Price: 10}))

	// статика — в памяти
	static := memory.New()
	static.SeedPrincipal(models.TeamPrincipal{ID: 5, Price: 10})
	static.SeedEngine(models.Engine{Engine: models.Mercedes, Price: 30, BaseLevel: 70})

	svc := service.New(static, dyn, nil, nil, nil)

	require.NoError(t, svc.StartDraftEconomy(ctx, g, []int64{1}))

	// игрок 1 собирает ростер: команда (Mercedes client = +10 → 40), 2 пилота (по 15), принципал (10)
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, g, dto.Draft{Pick: dto.DraftTeam, ItemID: 100, Engine: ice(models.Mercedes)}))
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, g, dto.Draft{Pick: dto.DraftPilot, ItemID: 1000}))
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, g, dto.Draft{Pick: dto.DraftPilot, ItemID: 1001}))
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, g, dto.Draft{Pick: dto.DraftPrincipal, ItemID: 5}))
	require.NoError(t, svc.AutoFillAfterDraft(ctx, g))

	// проверяем состояние в Redis
	p, err := dyn.GetPlayer(ctx, 1, g)
	require.NoError(t, err)
	require.Equal(t, int64(100), p.Team)
	require.NotNil(t, p.TeamPrincipal)
	// бюджет: 150 - 0 - 40 (мотор) - 15 - 15 (пилоты) - 10 (принципал) = 70
	require.Equal(t, 70, p.Budget)

	t100, err := dyn.GetPilotsByTeam(ctx, 100, g)
	require.NoError(t, err)
	require.Len(t, t100, 2)

	// бот укомплектован свободными пилотами
	t200, err := dyn.GetPilotsByTeam(ctx, 200, g)
	require.NoError(t, err)
	require.Len(t, t200, 2)

	free, err := dyn.GetUnassignedPilots(ctx, g)
	require.NoError(t, err)
	require.Empty(t, free)
}
