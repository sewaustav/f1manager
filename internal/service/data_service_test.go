package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"

	"github.com/stretchr/testify/require"
)

func TestGetMyTeamService(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	own := int64(1)
	gar := int64(100)
	pr := int64(5)
	r.SeedPlayer(g, models.Player{ID: 1, Team: 100, TeamPrincipal: &pr})
	r.SeedTeam(g, models.Team{ID: 100, Name: "Ferrari"})
	r.SeedPilot(g, models.Pilot{ID: 1000, Name: "A", Team: &own, Garage: &gar})
	r.SeedPilot(g, models.Pilot{ID: 1001, Name: "B", Team: &own, Garage: &gar})
	r.SeedPrincipal(models.TeamPrincipal{ID: 5, Name: "Boss", Price: 10})

	svc := New(r, r, nil, nil, nil)

	mt, err := svc.GetMyTeamService(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Ferrari", mt.Team.Name)
	require.Equal(t, "Boss", mt.TeamPrincipal.Name)
	require.NotEmpty(t, mt.Pilot1.Name)
	require.NotEmpty(t, mt.Pilot2.Name)
}

// Before a player has picked a team in the draft (Team == 0), buildMyTeam
// must not try to look up team 0 in Redis — that key never exists, and used
// to surface as a raw "redis: not found" error on the My Team screen and on
// /players/squads for the whole group (one team-less player broke everyone's
// view).
func TestGetMyTeamService_NoTeamPickedYet(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	r.SeedPlayer(g, models.Player{ID: 1})

	svc := New(r, r, nil, nil, nil)

	mt, err := svc.GetMyTeamService(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(0), mt.Team.ID)
	require.Empty(t, mt.Team.Name)
}

func TestGetPlayersTeamsService(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	own1, own2 := int64(1), int64(2)
	g100, g200 := int64(100), int64(200)
	r.SeedPlayer(g, models.Player{ID: 1, Team: 100})
	r.SeedPlayer(g, models.Player{ID: 2, Team: 200})
	r.SeedTeam(g, models.Team{ID: 100, Name: "Ferrari"})
	r.SeedTeam(g, models.Team{ID: 200, Name: "McLaren"})
	r.SeedPilot(g, models.Pilot{ID: 1000, Name: "A", Team: &own1, Garage: &g100})
	r.SeedPilot(g, models.Pilot{ID: 1001, Name: "B", Team: &own1, Garage: &g100})
	r.SeedPilot(g, models.Pilot{ID: 1002, Name: "C", Team: &own2, Garage: &g200})

	svc := New(r, r, nil, nil, nil)

	squads, err := svc.GetPlayersTeamsService(ctx, 1)
	require.NoError(t, err)
	require.Len(t, squads, 2)

	names := map[string]bool{}
	for _, s := range squads {
		names[s.Team.Name] = true
	}
	require.True(t, names["Ferrari"])
	require.True(t, names["McLaren"])
}

// /pilots used to be a flat static list with no group context, so it never
// reflected which pilots were already drafted — every pilot always showed
// as pickable even after being taken. GetPilotsService must return the
// group-scoped pilots (with Team populated for already-drafted ones).
func TestGetPilotsService_IsGroupScopedWithDraftStatus(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)
	owner := int64(1)

	r.SeedPlayer(g, models.Player{ID: 1})
	r.SeedPilot(g, models.Pilot{ID: 1000, Name: "Taken", Team: &owner})
	r.SeedPilot(g, models.Pilot{ID: 1001, Name: "Free"})

	svc := New(r, r, nil, nil, nil)

	pilots, err := svc.GetPilotsService(ctx, 1)
	require.NoError(t, err)
	require.Len(t, pilots, 2)

	byName := map[string]models.Pilot{}
	for _, p := range pilots {
		byName[p.Name] = p
	}
	require.NotNil(t, byName["Taken"].Team)
	require.Nil(t, byName["Free"].Team)
}

func TestGetTrackInfoService(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	// GetTracks у memory наследуется от stub (not implemented) — проверяем через реальную статику невозможно;
	// поэтому здесь проверяем только фильтрацию на пустом результате не имеет смысла.
	// Вместо этого убеждаемся, что метод корректно прокидывает ошибку статики.
	svc := New(r, r, nil, nil, nil)
	_, err := svc.GetTrackInfoService(ctx, "Monza")
	require.Error(t, err) // stub GetTracks → not implemented
}
