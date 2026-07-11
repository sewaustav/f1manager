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
