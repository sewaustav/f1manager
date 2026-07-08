package service

import "f1/internal/web/connection"

type SessionProvider interface {
	GetSession(userID int64) (*connection.Session, bool)
}
