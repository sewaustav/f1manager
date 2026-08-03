package service

import (
	"context"
	"errors"
	"fmt"

	"f1/internal/models"
	"f1/internal/web/dto"
)

// PilotTransfer — покупка пилота.
//
// Свободный агент покупается сразу. Пилот другого игрока не отбирается: мы
// создаём предложение, которое лежит на сервере, пока владелец не ответит.
// Раньше подтверждение ждали прямо в HTTP-запросе (до 60 секунд) по WS, и
// если у владельца не было живого сокета — обмен был невозможен в принципе.
func (s *Service) PilotTransfer(ctx context.Context, buyerUserID int64, req dto.PilotTransfer) error {
	groupID, err := s.getUserGroup(ctx, buyerUserID)
	if err != nil {
		return err
	}

	// Владельца берём из состояния группы, а не из статического шаблона
	// pilots_initial: там владельца нет никогда, из-за чего любой обмен
	// уходил по ветке свободного агента.
	pilot, err := s.dynamic.GetPilotByGroup(ctx, req.PilotID, groupID)
	if err != nil {
		return err
	}

	budget, err := s.dynamic.GetBudget(ctx, buyerUserID, groupID)
	if err != nil {
		return err
	}
	if budget < req.Price {
		return errors.New("недостаточно бюджета")
	}

	owned, err := s.dynamic.GetPlayerPilots(ctx, buyerUserID, groupID)
	if err != nil {
		return err
	}
	if len(owned) >= 2 {
		return errors.New("уже выбрано 2 пилота — сначала освободите место")
	}

	// Свободный агент — покупаем без подтверждения.
	if pilot.Team == nil {
		if err := s.dynamic.ExecutePilotTransfer(ctx, req.PilotID, 0, buyerUserID, req.Price); err != nil {
			return err
		}
		return s.dynamic.UpdateBudget(ctx, buyerUserID, groupID, -req.Price)
	}

	ownerUserID := *pilot.Team
	if ownerUserID == buyerUserID {
		return errors.New("этот пилот уже ваш")
	}

	// Не плодим дубликаты: одно открытое предложение на пилота от покупателя.
	offers, err := s.dynamic.ListTransferOffers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, o := range offers {
		if o.PilotID == req.PilotID && o.BuyerID == buyerUserID {
			return errors.New("предложение по этому пилоту уже отправлено")
		}
	}

	buyer, err := s.dynamic.GetPlayer(ctx, buyerUserID, groupID)
	if err != nil {
		return err
	}
	buyerName := buyer.Name
	if buyerName == "" {
		buyerName = fmt.Sprintf("Игрок %d", buyerUserID)
	}

	_, err = s.dynamic.CreateTransferOffer(ctx, groupID, models.TransferOffer{
		PilotID:   req.PilotID,
		PilotName: pilot.Name,
		BuyerID:   buyerUserID,
		BuyerName: buyerName,
		OwnerID:   ownerUserID,
		Price:     req.Price,
	})
	return err
}

// ListIncomingOffers — предложения, адресованные этому игроку.
func (s *Service) ListIncomingOffers(ctx context.Context, userID int64) ([]models.TransferOffer, error) {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return nil, err
	}
	all, err := s.dynamic.ListTransferOffers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	mine := make([]models.TransferOffer, 0, len(all))
	for _, o := range all {
		if o.OwnerID == userID {
			mine = append(mine, o)
		}
	}
	return mine, nil
}

// RespondToOffer — владелец принимает или отклоняет предложение. При принятии
// пилот и деньги меняются местами; в любом случае предложение закрывается.
func (s *Service) RespondToOffer(ctx context.Context, userID, offerID int64, accept bool) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}
	offer, err := s.dynamic.GetTransferOffer(ctx, groupID, offerID)
	if err != nil {
		return err
	}
	if offer.OwnerID != userID {
		return errors.New("это предложение адресовано не вам")
	}
	if !accept {
		return s.dynamic.DeleteTransferOffer(ctx, groupID, offerID)
	}

	// Условия перепроверяем на момент ответа: с отправки предложения игрок
	// мог потратить бюджет или лишиться пилота.
	pilot, err := s.dynamic.GetPilotByGroup(ctx, offer.PilotID, groupID)
	if err != nil {
		return err
	}
	if pilot.Team == nil || *pilot.Team != userID {
		_ = s.dynamic.DeleteTransferOffer(ctx, groupID, offerID)
		return errors.New("пилот вам больше не принадлежит")
	}
	buyerBudget, err := s.dynamic.GetBudget(ctx, offer.BuyerID, groupID)
	if err != nil {
		return err
	}
	if buyerBudget < offer.Price {
		_ = s.dynamic.DeleteTransferOffer(ctx, groupID, offerID)
		return errors.New("у покупателя больше нет нужного бюджета")
	}

	if err := s.dynamic.ExecutePilotTransfer(ctx, offer.PilotID, userID, offer.BuyerID, offer.Price); err != nil {
		return err
	}
	if err := s.dynamic.UpdateBudget(ctx, offer.BuyerID, groupID, -offer.Price); err != nil {
		return err
	}
	if err := s.dynamic.UpdateBudget(ctx, userID, groupID, offer.Price); err != nil {
		return err
	}
	return s.dynamic.DeleteTransferOffer(ctx, groupID, offerID)
}

// LeaveGroup выводит игрока из группы вместе с его открытыми предложениями.
func (s *Service) LeaveGroup(ctx context.Context, userID int64) error {
	groupID, err := s.getUserGroup(ctx, userID)
	if err != nil {
		return err
	}
	offers, err := s.dynamic.ListTransferOffers(ctx, groupID)
	if err != nil {
		return err
	}
	for _, o := range offers {
		if o.BuyerID == userID || o.OwnerID == userID {
			if err := s.dynamic.DeleteTransferOffer(ctx, groupID, o.ID); err != nil {
				return err
			}
		}
	}
	return s.dynamic.LeaveGroup(ctx, userID, groupID)
}
