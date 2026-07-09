package service

import "f1/internal/web/connection"

// SessionProvider позволяет сервисному слою получить сессию пользователя
// для отправки уведомлений и подписки на входящие сообщения.
type SessionProvider interface {
	GetSession(userID int64) (*connection.Session, bool)
}