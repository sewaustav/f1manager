package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

const grp = int64(1)

// newDraftFixture поднимает сервис на in-memory репозитории с двумя игроками,
// клиентской и заводской командой, пилотами, принципалом и моторами.
func newDraftFixture(t *testing.T) (*Service, *memory.Repo) {
	t.Helper()
	r := memory.New()

	r.SeedPlayer(grp, models.Player{ID: 1, Budget: draftVirtualBudget})
	r.SeedPlayer(grp, models.Player{ID: 2, Budget: draftVirtualBudget})

	// команда-клиент 100 и заводская 200
	r.SeedTeam(grp, models.Team{ID: 100, Budget: 150, IsManufacturer: models.Client, ICE: models.Mercedes})
	r.SeedTeam(grp, models.Team{ID: 200, Budget: 150, IsManufacturer: models.Manufacture, ICE: models.Ferrari})

	// пилоты: рейтинги для проверки автозаполнения
	r.SeedPilot(grp, models.Pilot{ID: 1000, Rating: 95, Price: 20, Sponsors: 5})
	r.SeedPilot(grp, models.Pilot{ID: 1001, Rating: 90, Price: 20, Sponsors: 5})
	r.SeedPilot(grp, models.Pilot{ID: 1002, Rating: 80, Price: 10, Sponsors: 0})

	r.SeedPrincipal(models.TeamPrincipal{ID: 5, Price: 10})

	r.SeedEngine(models.Engine{Engine: models.Ferrari, Price: 30})
	r.SeedEngine(models.Engine{Engine: models.Mercedes, Price: 30})

	svc := New(r, r, nil, nil, nil)
	return svc, r
}

func ice(v models.ICEName) *models.ICEName { return &v }

func TestStartDraftEconomy(t *testing.T) {
	ctx := context.Background()
	svc, r := newDraftFixture(t)
	require.NoError(t, svc.StartDraftEconomy(ctx, grp, []int64{1, 2}))
	b, _ := r.GetBudget(ctx, 1, grp)
	require.Equal(t, 110, b)
}

func TestApplyPilotThenTeamReconciliation(t *testing.T) {
	ctx := context.Background()
	svc, r := newDraftFixture(t)
	require.NoError(t, svc.StartDraftEconomy(ctx, grp, []int64{1, 2}))

	// игрок 1 берёт пилота 1000 (стоимость 20-5=15) на виртуальные 110 → 95
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftPilot, ItemID: 1000}))
	b, _ := r.GetBudget(ctx, 1, grp)
	require.Equal(t, 95, b)

	// затем берёт клиентскую команду 100 c мотором Mercedes (price+10 = 40)
	// newBudget = team.Budget(150) - spent(110-95=15) - engineCost(40) = 95
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftTeam, ItemID: 100, Engine: ice(models.Mercedes)}))
	b, _ = r.GetBudget(ctx, 1, grp)
	require.Equal(t, 95, b)

	// пилот 1000 получил гараж команды 100
	byTeam, _ := r.GetPilotsByTeam(ctx, 100, grp)
	require.Len(t, byTeam, 1)
	require.Equal(t, int64(1000), byTeam[0].ID)
}

func TestFactoryEngineForced(t *testing.T) {
	ctx := context.Background()
	svc, r := newDraftFixture(t)
	require.NoError(t, svc.StartDraftEconomy(ctx, grp, []int64{1, 2}))

	// игрок 1 берёт заводскую команду 200, пытается поставить Mercedes → форс Ferrari, цена = price (30, без +10)
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftTeam, ItemID: 200, Engine: ice(models.Mercedes)}))
	team, _ := r.GetTeamByGroup(ctx, 200, grp)
	require.Equal(t, models.Ferrari, team.ICE)
	b, _ := r.GetBudget(ctx, 1, grp)
	// newBudget = 150 - (110-110) - 30 = 120
	require.Equal(t, 120, b)
}

func TestDraftLimitsAndAvailability(t *testing.T) {
	ctx := context.Background()
	svc, r := newDraftFixture(t)
	require.NoError(t, svc.StartDraftEconomy(ctx, grp, []int64{1, 2}))

	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftPilot, ItemID: 1000}))
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftPilot, ItemID: 1001}))
	// третий пилот — превышение лимита
	err := svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftPilot, ItemID: 1002})
	require.Error(t, err)

	// пилот 1000 занят игроком 1 — игрок 2 не может его взять
	err = svc.ApplyDraftPick(ctx, 2, grp, dto.Draft{Pick: dto.DraftPilot, ItemID: 1000})
	require.Error(t, err)

	// команду 100 берёт игрок 1
	require.NoError(t, svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftTeam, ItemID: 100, Engine: ice(models.Mercedes)}))
	// вторую команду тот же игрок взять не может
	err = svc.ApplyDraftPick(ctx, 1, grp, dto.Draft{Pick: dto.DraftTeam, ItemID: 200, Engine: ice(models.Ferrari)})
	require.Error(t, err)
	// команда 100 занята — игрок 2 не может её взять
	err = svc.ApplyDraftPick(ctx, 2, grp, dto.Draft{Pick: dto.DraftTeam, ItemID: 100, Engine: ice(models.Mercedes)})
	require.Error(t, err)

	_ = r
}

func TestListGroupPlayers(t *testing.T) {
	ctx := context.Background()
	svc, _ := newDraftFixture(t)
	ids, err := svc.ListGroupPlayers(ctx, grp)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{1, 2}, ids)
}
