# Auth-модуль, миграции и сборка сервера — дизайн

Дата: 2026-07-09

## Цель

Добавить в f1manager модуль аутентификации (регистрация, логин, refresh, logout) на JWT RS256,
обновить миграции (таблицы `users` и `refresh_tokens`, `players.id` → FK на `users`),
реализовать сборку приложения в пакете `internal/server` и зарегистрировать все роутеры через gin.

## Решения, принятые с пользователем

- Идентификация: **email + username + password**; логин по email **или** username (одно поле `login`).
- Logout: **отзыв всех refresh-сессий** пользователя; access-токены доживают свой срок (6ч). Без blacklist/token_version.
- **Без ролей** в токене — только `sub` (user id).
- RSA-ключи: **PEM-файлы, пути из env** (`JWT_PRIVATE_KEY_PATH`, `JWT_PUBLIC_KEY_PATH`).
- StaticRepo/DynamicRepo: **заглушки** (методы возвращают "not implemented"), auth-репозиторий — полноценный Postgres.
- Entrypoint не создаём: только `server.New()/Run()`, main пользователь добавит сам.

## Структура

```
internal/auth/
├── model/model.go        User, RegisterRequest, LoginRequest, RefreshRequest, TokenPair
├── repo/postgres.go      Postgres-реализация AuthRepo (users + refresh_tokens)
├── service/service.go    интерфейсы AuthService и AuthRepo
├── service/auth.go       реализация Register, Login, Refresh, Logout
└── handler/
    ├── handler.go        AuthHandler + RegisterRoutes
    └── http.go           POST /auth/register, /auth/login, /auth/refresh, /auth/logout

pkg/middleware/jwt/
├── middleware.go         JWTAuthMiddleware (по образцу CaseGo, RS256, issuer/audience, slog)
└── claims.go             tokenClaims, Claims{UserID int64}, verifyToken — без ролей

internal/config/config.go env-конфиг: HTTP-порт, Postgres, пути к ключам, issuer/audience, TTL
internal/server/server.go New()/Run(): сборка графа + регистрация роутов
```

## Токены

- **Access**: JWT RS256, TTL 6h. Claims: `sub` (user id, строкой), `iss`, `aud`, `iat`, `exp`, `jti`.
- **Refresh**: 32 случайных байта → base64url. В БД хранится только SHA-256-хэш. TTL 30 дней (720h).
- **Ротация**: `/auth/refresh` отзывает использованную сессию и выдаёт новую пару.
  Попытка использовать отозванный/просроченный refresh → 401.
- **Logout**: по access-токену (middleware) определяется user id, все его refresh-сессии помечаются revoked.
- Пароли: bcrypt (DefaultCost).

## Миграции

Миграции ещё не прогонялись, поэтому правим существующую `20260702185021_initial_migration.up.sql`
(отдельный файл `20260709_auth` не создаём — `players` ссылается на `users`,
значит `users` обязана создаваться раньше в том же порядке выполнения):

- В начало файла: `users` (id BIGSERIAL PK, email TEXT UNIQUE NOT NULL, username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL, created_at TIMESTAMPTZ DEFAULT now()).
- `refresh_tokens` (id BIGSERIAL PK, user_id BIGINT NOT NULL FK→users, token_hash TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL, revoked BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ DEFAULT now())
  + индекс по user_id.
- `players`: `id BIGSERIAL` → `id BIGINT PRIMARY KEY REFERENCES users(id)` (не автоинкремент).
- Обновить `.down.sql` (drop в обратном порядке).

## HTTP-слой

- Валидация — через binding-теги в DTO и middleware, как в примере CaseGo.
- Роуты в `server`:
  - `/api/v1/auth`: register, login, refresh — публичные; logout — под `middleware.Handler()`.
  - `/api/v1` (игровые, все под middleware): `/ws`, `POST /setup`, `GET /race-result`, `GET /standing`,
    updates/transfers/draft (cross-season), data-эндпоинты, `POST /groups`, `POST /groups/join`,
    `POST /rounds/:stage/init`.
- CORS как в примере (AllowCredentials, Authorization в заголовках); origin из конфига.
- `getUser` в `internal/web/handler/http/utils.go` читает `sub` из gin-контекста (кладёт middleware).
- Ошибки auth-хендлеров: 400 на кривой JSON, 401 на неверные креды/токены, 409 на занятые email/username, 500 остальное.
- Известное ограничение: `HandleWs` под Bearer-middleware; браузерный WS не умеет ставить
  Authorization-заголовок — при необходимости позже добавим токен через query-параметр.

## Сборка (internal/server)

`server.New(cfg)` строит граф: db (`internal/db`) → RSA-ключи из PEM → auth repo/service/handler →
jwt middleware → `connection.NewManager()` (он же Notifier и SessionProvider) → `engine.NewEngine(db)` →
in-memory `UpdateCache` (map+mutex) → `service.New(...)` → `dispatcher.New(...)` → `HttpHandler` → gin.
`Run(ctx)` — запуск http.Server с graceful shutdown.

Заглушки: пакет `internal/new_storage/pg_stub` (или аналогичный) с типами, реализующими
`StaticRepo`/`DynamicRepo`, все методы возвращают ошибку `not implemented`.
In-memory UpdateCache — простая рабочая реализация.

## Зависимости

`github.com/golang-jwt/jwt/v5`, `github.com/gin-contrib/cors`, `golang.org/x/crypto` (bcrypt, станет прямой).

## Тестирование

- Unit-тесты auth-сервиса с фейковым AuthRepo: register (дубликаты), login (неверный пароль),
  refresh (ротация, повторное использование отозванного, просроченный), logout (отзыв всех сессий).
- Тест middleware: валидный/просроченный/чужим ключом подписанный токен.
- Компиляция всего проекта (`go build ./...`) и `go vet`.

## Вне объёма

- Реализация Postgres StaticRepo/DynamicRepo (только заглушки).
- Роли/права, восстановление пароля, rate limiting, blacklist access-токенов.
- Точка входа (main) для сервера.
