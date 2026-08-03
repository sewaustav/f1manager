package service

import (
	"context"
	"testing"

	"f1/internal/models"
	"f1/internal/new_storage/memory"
	"f1/internal/web/dto"

	"github.com/stretchr/testify/require"
)

// Двое игроков: 1 владеет пилотом 600, у 2 есть бюджет и свободный слот.
func transferFixture(t *testing.T) (*Service, *memory.Repo) {
	t.Helper()
	r := memory.New()
	const g = int64(1)
	owner := int64(1)

	r.SeedPlayer(g, models.Player{ID: 1, Name: "Первый", Team: 100, Budget: 10})
	r.SeedPlayer(g, models.Player{ID: 2, Name: "Второй", Team: 200, Budget: 50})
	r.SeedTeam(g, models.Team{ID: 100})
	r.SeedTeam(g, models.Team{ID: 200})
	r.SeedPilot(g, models.Pilot{ID: 600, Name: "Леклер", Team: &owner, Price: 20})
	r.SeedPilot(g, models.Pilot{ID: 601, Name: "Свободный", Price: 5})

	return New(r, r, nil, nil, nil), r
}

// Раньше владельца брали из static.GetPilot (таблица-шаблон, где владельца нет
// никогда), поэтому пилот другого игрока молча уходил по ветке свободного
// агента, а подтверждение висело на живом WS. Теперь создаётся предложение.
func TestPilotTransferFromAnotherPlayerCreatesAnOffer(t *testing.T) {
	ctx := context.Background()
	svc, r := transferFixture(t)

	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))

	// пилот пока не сменил владельца и деньги не ушли
	p, err := r.GetPilotByGroup(ctx, 600, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), *p.Team)
	budget, _ := r.GetBudget(ctx, 2, 1)
	require.Equal(t, 50, budget)

	offers, err := svc.ListIncomingOffers(ctx, 1)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, int64(600), offers[0].PilotID)
	require.Equal(t, "Леклер", offers[0].PilotName)
	require.Equal(t, int64(2), offers[0].BuyerID)
	require.Equal(t, "Второй", offers[0].BuyerName)
	require.Equal(t, 30, offers[0].Price)

	// покупателю его собственное предложение не показывается
	mine, err := svc.ListIncomingOffers(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, mine)
}

func TestRespondToOfferAcceptMovesPilotAndMoney(t *testing.T) {
	ctx := context.Background()
	svc, r := transferFixture(t)
	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))
	offers, _ := svc.ListIncomingOffers(ctx, 1)

	require.NoError(t, svc.RespondToOffer(ctx, 1, offers[0].ID, true))

	p, _ := r.GetPilotByGroup(ctx, 600, 1)
	require.Equal(t, int64(2), *p.Team, "пилот у покупателя")

	buyer, _ := r.GetBudget(ctx, 2, 1)
	seller, _ := r.GetBudget(ctx, 1, 1)
	require.Equal(t, 20, buyer, "50 - 30")
	require.Equal(t, 40, seller, "10 + 30")

	left, _ := svc.ListIncomingOffers(ctx, 1)
	require.Empty(t, left, "предложение закрыто")
}

func TestRespondToOfferDeclineChangesNothing(t *testing.T) {
	ctx := context.Background()
	svc, r := transferFixture(t)
	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))
	offers, _ := svc.ListIncomingOffers(ctx, 1)

	require.NoError(t, svc.RespondToOffer(ctx, 1, offers[0].ID, false))

	p, _ := r.GetPilotByGroup(ctx, 600, 1)
	require.Equal(t, int64(1), *p.Team)
	buyer, _ := r.GetBudget(ctx, 2, 1)
	require.Equal(t, 50, buyer)
	left, _ := svc.ListIncomingOffers(ctx, 1)
	require.Empty(t, left)
}

func TestRespondToOfferOnlyByTheOwner(t *testing.T) {
	ctx := context.Background()
	svc, _ := transferFixture(t)
	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))
	offers, _ := svc.ListIncomingOffers(ctx, 1)

	// покупатель не может принять собственное предложение
	err := svc.RespondToOffer(ctx, 2, offers[0].ID, true)
	require.Error(t, err)
}

func TestPilotTransferFreeAgentIsInstant(t *testing.T) {
	ctx := context.Background()
	svc, r := transferFixture(t)

	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 601, Price: 5}))

	p, _ := r.GetPilotByGroup(ctx, 601, 1)
	require.NotNil(t, p.Team)
	require.Equal(t, int64(2), *p.Team)
	budget, _ := r.GetBudget(ctx, 2, 1)
	require.Equal(t, 45, budget)
	require.Empty(t, mustOffers(t, svc, 1), "свободный агент не требует подтверждения")
}

func TestPilotTransferRejectsDuplicateOffer(t *testing.T) {
	ctx := context.Background()
	svc, _ := transferFixture(t)
	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))
	require.Error(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 35}))
}

func TestPilotTransferRejectsWhenBudgetTooLow(t *testing.T) {
	ctx := context.Background()
	svc, _ := transferFixture(t)
	require.Error(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 999}))
}

// Уход из группы освобождает пилотов игрока и снимает его открытые предложения.
func TestLeaveGroupFreesPilotsAndOffers(t *testing.T) {
	ctx := context.Background()
	svc, r := transferFixture(t)
	require.NoError(t, svc.PilotTransfer(ctx, 2, dto.PilotTransfer{PilotID: 600, Price: 30}))

	require.NoError(t, svc.LeaveGroup(ctx, 1))

	p, _ := r.GetPilotByGroup(ctx, 600, 1)
	require.Nil(t, p.Team, "пилот ушедшего снова свободен")

	players, _ := r.GetPlayers(ctx, 1)
	for _, pl := range players {
		require.NotEqual(t, int64(1), pl.ID, "игрок убран из группы")
	}

	all, err := r.ListTransferOffers(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, all, "его предложения закрыты")
}

func mustOffers(t *testing.T, svc *Service, userID int64) []models.TransferOffer {
	t.Helper()
	o, err := svc.ListIncomingOffers(context.Background(), userID)
	require.NoError(t, err)
	return o
}
