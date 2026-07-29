package memory

import (
	"context"
	"testing"

	"f1/internal/models"

	"github.com/stretchr/testify/require"
)

func ptr(v int64) *int64 { return &v }

func TestMemoryRepoDraftMethods(t *testing.T) {
	ctx := context.Background()
	r := New()

	const g = int64(1)
	r.SeedPlayer(g, models.Player{ID: 10, Budget: 110})
	r.SeedTeam(g, models.Team{ID: 100, Budget: 150, ICE: models.Ferrari})
	r.SeedPilot(g, models.Pilot{ID: 1000, Rating: 90, Price: 20, Sponsors: 5})

	// свободный пилот
	free, err := r.GetUnassignedPilots(ctx, g)
	require.NoError(t, err)
	require.Len(t, free, 1)

	// бюджет
	b, err := r.GetBudget(ctx, 10, g)
	require.NoError(t, err)
	require.Equal(t, 110, b)
	require.NoError(t, r.SetPlayerBudget(ctx, 10, g, 70))
	b, _ = r.GetBudget(ctx, 10, g)
	require.Equal(t, 70, b)

	// назначить команду игроку → команда больше не бот
	require.NoError(t, r.SetPlayerTeam(ctx, 10, g, 100))
	bots, err := r.GetBotTeams(ctx, g)
	require.NoError(t, err)
	require.Empty(t, bots)

	// назначить пилота игроку (owner=10) с гаражом команды 100
	require.NoError(t, r.SetPilotOwner(ctx, 1000, g, ptr(10), ptr(100)))
	owned, err := r.GetPlayerPilots(ctx, 10, g)
	require.NoError(t, err)
	require.Len(t, owned, 1)
	byTeam, err := r.GetPilotsByTeam(ctx, 100, g)
	require.NoError(t, err)
	require.Len(t, byTeam, 1)
	free, _ = r.GetUnassignedPilots(ctx, g)
	require.Empty(t, free)

	// принципал
	r.SeedPrincipal(models.TeamPrincipal{ID: 5, Price: 10})
	pr, err := r.GetTeamPrincipal(ctx, 5)
	require.NoError(t, err)
	require.Equal(t, 10, pr.Price)
	require.NoError(t, r.SetPlayerPrincipal(ctx, 10, g, 5))
	p, _ := r.GetPlayer(ctx, 10, g)
	require.NotNil(t, p.TeamPrincipal)
	require.Equal(t, int64(5), *p.TeamPrincipal)

	// мотор
	r.SeedEngine(models.Engine{Engine: models.Ferrari, Price: 30})
	require.NoError(t, r.SetTeamEngine(ctx, 100, g, models.Mercedes))
	team, _ := r.GetTeamByGroup(ctx, 100, g)
	require.Equal(t, models.Mercedes, team.ICE)
	engines, err := r.GetEngines(ctx)
	require.NoError(t, err)
	require.Len(t, engines, 1)
}
