package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

func TestChooseSetupPreservesTokens(t *testing.T) {
	ctx := context.Background()
	r := memory.New()
	const g = int64(1)

	r.SeedPlayer(g, models.Player{ID: 1, Team: 100, Tokens: 120})
	r.SeedTeam(g, models.Team{ID: 100})

	svc := New(r, r, nil, nil, nil)

	setup := dto.Setup{AeroDynamic: 10, Engine: 10, Chassis: 10, Floor: 10, Tyres: 10, Reliability: 10}
	require.NoError(t, svc.ChooseSetup(ctx, 1, setup))

	// баланс токенов сохранился (не ушёл в 120-60=60)
	tokens, err := r.GetTokens(ctx, 1, g)
	require.NoError(t, err)
	require.Equal(t, 120, tokens)

	// распределение применено к болиду
	car, err := r.GetCar(ctx, 100, g)
	require.NoError(t, err)
	require.Equal(t, 10, car.AeroDynamic)
}
