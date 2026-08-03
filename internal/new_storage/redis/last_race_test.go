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

// HandleRace persists the results but used to leave the stage key unwritten,
// so GetLastRaceResults always reported stage 0. A client waiting on its race
// could not tell "my race finished" from a leftover earlier result and waited
// forever, and the mid-season update window (stages 3/8/13) never opened.
func TestHandleRaceRoundTripsTheStage(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	dyn := redisrepo.NewDynamic(rdb)
	const g = int64(1)

	// no race yet — stage 0, nothing to show
	results, stage, err := dyn.GetLastRaceResults(ctx, g)
	require.NoError(t, err)
	require.Empty(t, results)
	require.Equal(t, int64(0), stage)

	race := []models.RaceResult{{PilotID: 7, PilotName: "Verstappen", RacePosition: 1, Points: 25}}
	require.NoError(t, dyn.HandleRace(ctx, race, g, 3))

	results, stage, err = dyn.GetLastRaceResults(ctx, g)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Verstappen", results[0].PilotName)
	require.Equal(t, int64(3), stage)

	// the next race overwrites both, so clients see the newest stage
	require.NoError(t, dyn.HandleRace(ctx, race, g, 4))
	_, stage, err = dyn.GetLastRaceResults(ctx, g)
	require.NoError(t, err)
	require.Equal(t, int64(4), stage)
}
