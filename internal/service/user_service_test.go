package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"

	"github.com/stretchr/testify/require"
)

// newUserFixture поднимает сервис на in-memory репозитории с двумя
// статическими шаблонами команд и пилотов, для проверки сидирования нового
// мира группы при RegisterGroup/ResetGroup.
func newUserFixture(t *testing.T) (*Service, *memory.Repo) {
	t.Helper()
	r := memory.New()
	r.SeedBaseTeams([]models.Team{
		{ID: 100, Name: "Ferrari", Budget: 150, IsManufacturer: models.Manufacture, ICE: models.Ferrari},
		{ID: 200, Name: "Mercedes", Budget: 150, IsManufacturer: models.Manufacture, ICE: models.Mercedes},
	})
	r.SeedStaticPilots([]models.Pilot{
		{ID: 1000, Name: "Verstappen", Rating: 97, Price: 50},
		{ID: 1001, Name: "Hamilton", Rating: 95, Price: 40},
	})
	svc := New(r, r, nil, nil, nil)
	return svc, r
}

func TestRegisterGroupSeedsPilotsAndTeams(t *testing.T) {
	ctx := context.Background()
	svc, r := newUserFixture(t)

	// seedGroupWorld is what RegisterGroup calls after dynamic.RegisterGroup
	// succeeds; tested directly here since memory.Repo doesn't implement the
	// group-creation half (RegisterGroup/JoinGroup) of DynamicRepo.
	require.NoError(t, svc.seedGroupWorld(ctx, 1))

	teams, err := r.GetTeamsByGroup(ctx, 1)
	require.NoError(t, err)
	require.Len(t, teams, 2, "the group's teams must be seeded from GetBaseTeams")
	var names []string
	for _, tm := range teams {
		names = append(names, tm.Name)
	}
	require.ElementsMatch(t, []string{"Ferrari", "Mercedes"}, names)

	pilots, err := r.GetPilotsByGroup(ctx, 1)
	require.NoError(t, err)
	require.Len(t, pilots, 2, "the group's pilots must be seeded from the static pilot list")

	// A freshly seeded group must actually be draftable: GetTeamByGroup/
	// GetPilotByGroup (what ApplyDraftPick reads) must resolve, not ErrNotFound.
	_, err = r.GetTeamByGroup(ctx, 100, 1)
	require.NoError(t, err)
	_, err = r.GetPilotByGroup(ctx, 1000, 1)
	require.NoError(t, err)
}

// TestResetGroup covers "end the game early" — previously the only option
// was logging out, which never touched any server-side state at all, so
// logging back in dropped the player right back into the same (possibly
// stuck) group.
func TestResetGroupClearsPlayersAndReseedsWorld(t *testing.T) {
	ctx := context.Background()
	svc, r := newUserFixture(t)

	principal := int64(5)
	r.SeedPlayer(1, models.Player{ID: 1, Team: 100, Budget: 42, TeamPrincipal: &principal})
	r.SeedPlayer(1, models.Player{ID: 2, Team: 200, Budget: 7})
	// dirty a pilot (as a draft pick would) to prove reset overwrites, not just adds
	owner := int64(1)
	r.SeedPilot(1, models.Pilot{ID: 1000, Name: "Verstappen", Team: &owner})

	require.NoError(t, svc.ResetGroup(ctx, 1))

	p1, err := r.GetPlayer(ctx, 1, 1)
	require.NoError(t, err)
	require.Equal(t, models.Player{ID: 1}, p1, "player fully reset: team/budget/principal all cleared")

	p2, err := r.GetPlayer(ctx, 2, 1)
	require.NoError(t, err)
	require.Equal(t, models.Player{ID: 2}, p2)

	pilots, err := r.GetPilotsByGroup(ctx, 1)
	require.NoError(t, err)
	require.Len(t, pilots, 2, "world is re-seeded from the static pilot list")
	for _, p := range pilots {
		require.Nil(t, p.Team, "re-seeding must overwrite the prior draft assignment")
	}

	teams, err := r.GetTeamsByGroup(ctx, 1)
	require.NoError(t, err)
	require.Len(t, teams, 2, "world is re-seeded from the base team templates")
}
