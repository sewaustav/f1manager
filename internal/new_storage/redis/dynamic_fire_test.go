package redis_test

import (
	"context"
	"testing"

	"f1/internal/models"
	redisrepo "f1/internal/new_storage/redis"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestDynamicFirePilot проверяет, что увольнение пилота отвязывает его от
// команды/гаража и возвращает игроку (price - sponsors) в бюджет.
func TestDynamicFirePilot(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	dyn := redisrepo.NewDynamic(rdb)

	const g = int64(1)
	const userID = int64(1)
	team := userID
	garage := userID

	require.NoError(t, dyn.SavePlayer(ctx, g, models.Player{ID: userID, Budget: 50}))
	require.NoError(t, dyn.SavePilot(ctx, g, models.Pilot{ID: 1000, Price: 20, Sponsors: 5, Team: &team, Garage: &garage}))

	require.NoError(t, dyn.Fire(ctx, userID, g, "pilot", 1000))

	pilot, err := dyn.GetPilotByGroup(ctx, 1000, g)
	require.NoError(t, err)
	require.Nil(t, pilot.Team)
	require.Nil(t, pilot.Garage)

	player, err := dyn.GetPlayer(ctx, userID, g)
	require.NoError(t, err)
	// 50 + (20 - 5) = 65
	require.Equal(t, 65, player.Budget)
}

// TestDynamicFirePrincipal проверяет, что увольнение принципала очищает
// player.TeamPrincipal. Рефанд статической цены выполняется на уровне
// Service.Fire (см. internal/service), т.к. DynamicRepo не имеет доступа
// к StaticRepo.
func TestDynamicFirePrincipal(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	dyn := redisrepo.NewDynamic(rdb)

	const g = int64(1)
	const userID = int64(1)
	principalID := int64(5)

	require.NoError(t, dyn.SavePlayer(ctx, g, models.Player{ID: userID, Budget: 50, TeamPrincipal: &principalID}))

	require.NoError(t, dyn.Fire(ctx, userID, g, "principal", principalID))

	player, err := dyn.GetPlayer(ctx, userID, g)
	require.NoError(t, err)
	require.Nil(t, player.TeamPrincipal)
}

// TestDynamicFireUnknownWho проверяет обработку некорректного значения who.
func TestDynamicFireUnknownWho(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	dyn := redisrepo.NewDynamic(rdb)
	require.Error(t, dyn.Fire(ctx, 1, 1, "bogus", 1))
}
