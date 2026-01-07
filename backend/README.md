# Telegram Webapp — Backend

Минимальный бэкенд для WebApp Telegram: аутентификация через init_data, JWT и WebSocket матчмейкинг для игры rock/paper/scissors.

Кратко:

- Запуск: установите `.env` (см. `.env.example`) и выполните `go run ./cmd/app`.
- Маршруты:

  - `POST /api/auth` — принять `init_data`, вернуть `token` и `user_id`.
  - `GET /api/me` — вернуть текущего пользователя (требует `Authorization: Bearer <token>`).
  - `GET /ws?token=<jwt>` — подключение WebSocket для матчмейкинга.

  Frontend demo:

  - see `frontend/index.html` — minimal page that connects to `/ws?token=<jwt>`.

Миграции находятся в `internal/migrations`.
