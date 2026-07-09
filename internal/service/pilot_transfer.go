package service

import (
	"context"
	"encoding/json"
	"errors"
	"f1/internal/web/dto"
	"time"
)

// transferRequestMsg — WS-сообщение владельцу пилота.
type transferRequestMsg struct {
	Type    string `json:"type"`
	PilotID int64  `json:"pilot_id"`
	Price   int    `json:"price"`
}

// transferResponseMsg — ответ от владельца.
type transferResponseMsg struct {
	Type    string `json:"type"`    // "transfer_response"
	PilotID int64  `json:"pilot_id"`
	Accept  bool   `json:"accept"`
}

const transferTimeout = 60 * time.Second

// PilotTransfer — покупка пилота у другого игрока или свободного агента.
//
// Если пилот принадлежит другому игроку — отправляем ему WS-уведомление
// и ждём подтверждения через Subscribe (не блокируем общий канал Messages).
func (s *Service) PilotTransfer(ctx context.Context, buyerUserID int64, req dto.PilotTransfer) error {
	pilot, err := s.static.GetPilot(ctx, req.PilotID)
	if err != nil {
		return err
	}

	groupID, err := s.getUserGroup(ctx, buyerUserID)
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

	// Свободный агент — покупаем без подтверждения.
	if pilot.Team == nil {
		if err := s.dynamic.ExecutePilotTransfer(ctx, req.PilotID, 0, buyerUserID, req.Price); err != nil {
			return err
		}
		return s.dynamic.UpdateBudget(ctx, buyerUserID, groupID, -req.Price)
	}

	// Пилот принадлежит другому игроку — нужно подтверждение.
	ownerUserID, err := s.getOwnerByTeam(ctx, *pilot.Team, groupID)
	if err != nil {
		return err
	}

	session, ok := s.sessionProvider.GetSession(ownerUserID)
	if !ok {
		return errors.New("владелец пилота не в сети")
	}

	// Отправляем уведомление владельцу.
	msg, _ := json.Marshal(transferRequestMsg{
		Type:    "transfer_request",
		PilotID: req.PilotID,
		Price:   req.Price,
	})
	session.Send(msg)

	// Подписываемся на ответ — Subscribe не конкурирует с другими читателями.
	responseCh := make(chan transferResponseMsg, 1)

	unsubscribe := session.Subscribe(func(raw []byte) {
		var resp transferResponseMsg
		if err := json.Unmarshal(raw, &resp); err != nil {
			return
		}
		if resp.Type != "transfer_response" || resp.PilotID != req.PilotID {
			return
		}
		select {
		case responseCh <- resp:
		default:
		}
	})
	defer unsubscribe()

	// Ждём ответа, таймаута или разрыва соединения.
	timer := time.NewTimer(transferTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return errors.New("время ожидания подтверждения истекло")

	case <-session.Done():
		return errors.New("владелец отключился до подтверждения")

	case resp := <-responseCh:
		if !resp.Accept {
			return errors.New("владелец отклонил трансфер")
		}
	}

	// Подтверждено — выполняем трансфер.
	if err := s.dynamic.ExecutePilotTransfer(ctx, req.PilotID, *pilot.Team, buyerUserID, req.Price); err != nil {
		return err
	}
	if err := s.dynamic.UpdateBudget(ctx, buyerUserID, groupID, -req.Price); err != nil {
		return err
	}
	if err := s.dynamic.UpdateBudget(ctx, ownerUserID, groupID, req.Price); err != nil {
		return err
	}

	return nil
}