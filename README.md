# Telegram WebApp Gaming Platform

Игровая платформа для Telegram с PvE и PvP играми, системой квестов, TON платежами и лидербордом.

---

## Содержание

- [Архитектура](#архитектура)
- [Backend - Реализованные функции](#backend---реализованные-функции)
- [База данных](#база-данных)
- [Запуск проекта](#запуск-проекта)
- [Переменные окружения](#переменные-окружения)

---

## Архитектура

```
telegram-webapp/
├── backend/                 # Go + Gin + PostgreSQL
│   ├── cmd/
│   │   ├── app/            # Основной сервер
│   │   ├── migrate_apply/  # Миграции БД
│   │   └── ws_smoke/       # Smoke тесты WebSocket
│   ├── internal/
│   │   ├── bot/            # Telegram Admin Bot
│   │   ├── config/         # Конфигурация
│   │   ├── db/             # Подключение к БД
│   │   ├── domain/         # Модели данных
│   │   ├── game/           # Игровая логика
│   │   ├── http/
│   │   │   ├── handlers/   # API хендлеры
│   │   │   └── middleware/ # JWT, Rate Limiting
│   │   ├── logger/         # Structured logging
│   │   ├── repository/     # Работа с БД
│   │   ├── service/        # Бизнес-логика
│   │   ├── telegram/       # Telegram Auth
│   │   ├── ton/            # TON интеграция
│   │   └── ws/             # WebSocket для PvP
│   └── migrations/         # SQL миграции
│
├── frontend/               # Собранный React билд
└── frontend-new/           # React + Vite + Tailwind (исходники)
```

**Технологии:**
- Backend: Go 1.21+, Gin, pgx/v5, JWT, WebSocket, Prometheus
- Frontend: React 18, Vite 5, Tailwind CSS 3
- Database: PostgreSQL 15+
- Cache/RateLimit: Redis (опционально)
- Payments: TON Blockchain

---

## Backend - Реализованные функции

### API Endpoints (40+ эндпоинтов)

#### Health Checks
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/health` | Полная проверка (DB, версия) |
| GET | `/healthz` | Liveness probe для K8s |
| GET | `/readyz` | Readiness probe для K8s |
| GET | `/metrics` | Prometheus метрики |

#### Аутентификация
| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/api/v1/auth` | Авторизация через Telegram initData |

- Валидация HMAC-SHA256 подписи Telegram
- Генерация JWT токена (24 часа)
- DEV_MODE для тестирования без Telegram

#### Профиль пользователя
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/me` | Базовая информация о пользователе |
| GET | `/api/v1/profile` | Профиль с балансом и транзакциями |
| POST | `/api/v1/profile/balance` | Изменение баланса |
| POST | `/api/v1/profile/bonus` | Получить бонус |
| GET | `/api/v1/profile/:id` | Публичный профиль пользователя |

#### PvE Игры
| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/api/v1/game/coinflip` | Coin Flip - 50/50 шанс, x2 |
| POST | `/api/v1/game/rps` | Rock Paper Scissors vs Bot |
| POST | `/api/v1/game/mines` | Mines - 8 safe / 4 mines, x2 |
| POST | `/api/v1/game/case` | Case - лутбокс (100 gems) |
| POST | `/api/v1/game/dice` | Dice - настраиваемый шанс/множитель |
| GET | `/api/v1/game/dice/info` | Информация о Dice |
| POST | `/api/v1/game/wheel` | Wheel of Fortune |
| GET | `/api/v1/game/wheel/info` | Информация о Wheel |

#### Mines Pro (Продвинутая версия Mines)
| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/api/v1/game/mines-pro/start` | Начать игру (5x5 поле, 1-24 мины) |
| POST | `/api/v1/game/mines-pro/reveal` | Открыть ячейку |
| POST | `/api/v1/game/mines-pro/cashout` | Забрать выигрыш |
| GET | `/api/v1/game/mines-pro/state` | Текущее состояние игры |
| GET | `/api/v1/game/mines-pro/info` | Таблицы множителей |

#### Лимиты игр
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/game/limits` | Мин/макс ставки |

#### Статистика и история
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/me/games` | История игр + статистика |
| GET | `/api/v1/top` | Топ-50 игроков по победам |
| GET | `/api/v1/history` | История транзакций |
| POST | `/api/v1/history` | Записать транзакцию |

#### Квесты
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/quests` | Список активных квестов |
| GET | `/api/v1/me/quests` | Прогресс квестов пользователя |
| POST | `/api/v1/quests/:id/claim` | Забрать награду за квест |

#### TON Connect & Payments
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/ton/config` | Конфигурация TON Connect |
| GET | `/api/v1/ton/wallet` | Информация о кошельке |
| POST | `/api/v1/ton/wallet/connect` | Подключить кошелёк |
| DELETE | `/api/v1/ton/wallet` | Отключить кошелёк |
| GET | `/api/v1/ton/deposit/info` | Информация для депозита |
| GET | `/api/v1/ton/deposits` | История депозитов |
| POST | `/api/v1/ton/deposit/manual` | Ручной депозит (dev) |
| POST | `/api/v1/ton/withdraw/estimate` | Оценка вывода |
| POST | `/api/v1/ton/withdraw` | Запрос на вывод |
| GET | `/api/v1/ton/withdrawals` | История выводов |
| POST | `/api/v1/ton/withdraw/cancel` | Отмена вывода |

#### WebSocket (PvP)
| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/ws` | WebSocket для PvP игр |

Query параметры:
- `token=<jwt>` - JWT токен
- `game=rps|mines` - тип игры
- `bet=<amount>` - размер ставки
- `currency=gems|coins` - валюта

---

### Игровая механика

#### CoinFlip (PvE)
```
Ставка: MIN_BET - MAX_BET
Шанс: 50/50
Выигрыш: bet × 2
Проигрыш: -bet
```

#### Rock Paper Scissors (PvE)
```
Ставка: MIN_BET - MAX_BET (можно 0)
Механика: rock > scissors > paper > rock
Выигрыш: bet × 2
Ничья: возврат ставки
Проигрыш: -bet
```

#### Mines Simple (PvE)
```
Ставка: MIN_BET - MAX_BET
Поле: 12 ячеек (4 мины, 8 безопасных)
Выбор: 1 ячейка
Безопасно: bet × 2
Мина: -bet
```

#### Dice (PvE)
```
Ставка: MIN_BET - MAX_BET
Target: 1.00 - 98.99
Режимы: Roll Over / Roll Under
Множитель: 100 / win_chance (честные коэффициенты)
Примеры:
  - Target 50.00, Roll Over: 49.99% шанс, x2.00
  - Target 25.00, Roll Under: 25% шанс, x4.00
  - Target 10.00, Roll Under: 10% шанс, x10.00
```

#### Wheel of Fortune (PvE)
```
Ставка: MIN_BET - MAX_BET
Сегменты колеса с разными множителями:
  - x0.1 (красный) - частый
  - x0.5 (оранжевый)
  - x1.0 (жёлтый) - возврат
  - x1.5 (зелёный)
  - x2.0 (синий)
  - x3.0 (фиолетовый) - редкий
  - x10.0 (золотой) - очень редкий
```

#### Mines Pro (PvE - продвинутая версия)
```
Поле: 5x5 (25 ячеек)
Мины: 1-24 (выбор игрока)
Механика:
  1. Начать игру с выбором кол-ва мин
  2. Открывать ячейки - каждая безопасная увеличивает множитель
  3. Cashout в любой момент или попасть на мину
Множители: прогрессивные, зависят от кол-ва мин и открытых ячеек
```

#### Case/Roulette (Solo)
```
Стоимость: 100 gems
Призы:
├── Case 1: 250 gems  (50% шанс)
├── Case 2: 500 gems  (20% шанс)
├── Case 3: 750 gems  (15% шанс)
├── Case 4: 1000 gems (10% шанс)
└── Case 5: 5000 gems (5% шанс)
```

#### PvP RPS (WebSocket)
```
Матчмейкинг: автоматический по ставке
Таймаут хода: 20 секунд
Механика: оба игрока выбирают одновременно
Выигрыш: bet × 2 (ставки обоих игроков)
```

#### PvP Mines (WebSocket)
```
Фаза 1 - Setup (10 сек):
  Каждый игрок размещает 4 мины на своём поле 4x3

Фаза 2 - Playing (10 сек/ход, макс 5 раундов):
  Игроки одновременно выбирают ячейки соперника
  Попал на мину = проиграл

Результат:
  - Первый на мине проигрывает
  - Оба на мине = продолжение
  - 5 раундов без попаданий = ничья
Выигрыш: bet × 2
```

---

### WebSocket протокол

#### Client → Server
```json
{ "type": "move", "value": "rock" }           // RPS
{ "type": "move", "value": [1,2,3,4] }        // Mines setup (позиции мин)
{ "type": "move", "value": 5 }                // Mines pick (номер ячейки)
```

#### Server → Client
```json
{ "type": "ready" }
{ "type": "state", "payload": { "room_id": "...", "players": 2, "game_type": "mines" } }
{ "type": "matched", "payload": { "room_id": "...", "opponent": { "id": 123 } } }
{ "type": "start", "payload": { "timestamp": 1234567890 } }
{ "type": "round_result", "payload": { "round": 1, "your_move": 5, "your_hit": false, ... } }
{ "type": "round_draw", "payload": { "message": "..." } }
{ "type": "result", "payload": { "you": "win", "reason": "opponent_hit_mine", "win_amount": 200 } }
{ "type": "error", "payload": { "message": "..." } }
```

---

### Система валют

#### Gems (бесплатная валюта)
- Начальный баланс: 10,000 gems
- Зарабатываются: PvE игры, квесты, бонусы
- Используются: ставки в играх

#### Coins (премиум валюта)
- Курс: 10 coins = 1 TON
- Покупаются: депозит TON
- Выводятся: на TON кошелёк (комиссия 5%)
- Минимальный вывод: 10 coins (1 TON)

---

### Система квестов

**Типы квестов:**
- `daily` - сброс в полночь
- `weekly` - сброс каждые 7 дней
- `one_time` - выполняется один раз

**Действия:**
- `play` - сыграть N игр
- `win` - выиграть N раз
- `lose` - проиграть N раз
- `spend_gems` - потратить gems
- `earn_gems` - заработать gems

**Привязка к играм:**
`game_type`: `rps`, `mines`, `coinflip`, `case`, `dice`, `wheel`, `mines_pro`, `any`, или NULL

---

### Middleware

| Middleware | Функция |
|------------|---------|
| JWT Auth | Валидация Bearer токена |
| Redis Rate Limit | API: 10 req/min, Auth: 5 req/min |
| Game Rate Limit | 60 игр/мин на пользователя |
| Memory Rate Limit | Fallback если Redis недоступен |
| CORS | Cross-Origin для фронтенда |
| Metrics | Prometheus метрики |

---

### Admin Bot (Telegram)

Телеграм бот для администрирования:
- `/stats` - статистика платформы
- `/user <id>` - информация о пользователе
- `/balance <id> <amount>` - изменить баланс
- Уведомления о крупных транзакциях

---

### Audit Logging

Логирование всех важных событий:
- Игры (ставка, результат, выигрыш)
- Депозиты и выводы
- Изменения баланса
- Административные действия

---

## База данных

### Таблицы (13 миграций)

#### users
```sql
id          BIGSERIAL PRIMARY KEY
tg_id       BIGINT UNIQUE NOT NULL
username    VARCHAR(255)
first_name  VARCHAR(255)
gems        BIGINT DEFAULT 10000    -- Бесплатная валюта
coins       BIGINT DEFAULT 0        -- Премиум валюта
created_at  TIMESTAMP DEFAULT NOW()
```

#### game_history
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT REFERENCES users(id)
game_type   VARCHAR(50)             -- coinflip, rps, mines, dice, wheel, mines_pro
mode        VARCHAR(20)             -- pve, pvp, solo
opponent_id BIGINT                  -- для PvP
room_id     VARCHAR(100)            -- для PvP
result      VARCHAR(20)             -- win, lose, draw
bet_amount  BIGINT
win_amount  BIGINT
currency    VARCHAR(10)             -- gems, coins
details     JSONB
created_at  TIMESTAMP DEFAULT NOW()
```

#### transactions
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT REFERENCES users(id)
type        VARCHAR(50)             -- game, deposit, withdrawal, quest_reward
amount      BIGINT
meta        JSONB
created_at  TIMESTAMP DEFAULT NOW()
```

#### ton_wallets
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT UNIQUE REFERENCES users(id)
address     VARCHAR(100) NOT NULL
connected_at TIMESTAMP DEFAULT NOW()
```

#### ton_deposits
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT REFERENCES users(id)
wallet_address VARCHAR(100)
amount_nano BIGINT                  -- в нанотонах
amount_coins BIGINT                 -- в coins (10 per TON)
tx_hash     VARCHAR(100) UNIQUE
status      VARCHAR(20)             -- pending, confirmed, failed
created_at  TIMESTAMP DEFAULT NOW()
confirmed_at TIMESTAMP
```

#### ton_withdrawals
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT REFERENCES users(id)
wallet_address VARCHAR(100)
amount_coins BIGINT
amount_nano BIGINT
fee_coins   BIGINT
status      VARCHAR(20)             -- pending, processing, completed, failed, cancelled
tx_hash     VARCHAR(100)
created_at  TIMESTAMP DEFAULT NOW()
processed_at TIMESTAMP
```

#### audit_logs
```sql
id          BIGSERIAL PRIMARY KEY
user_id     BIGINT
action      VARCHAR(50)
details     JSONB
ip_address  VARCHAR(45)
created_at  TIMESTAMP DEFAULT NOW()
```

#### quests / user_quests
Система квестов с прогрессом и наградами.

#### games (legacy)
Старая таблица для PvP, сохранена для совместимости.

---

## Запуск проекта

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Redis (опционально, для rate limiting)

### Установка

```bash
# Clone
git clone <repo>
cd telegram-webapp

# Backend
cd backend
go mod download

# Создать .env файл
cp .env.example .env
# Заполнить переменные

# Миграции
go run cmd/migrate_apply/main.go

# Запуск
go run cmd/app/main.go
```

### Docker

```bash
docker-compose up -d
```

---

## Переменные окружения

### Обязательные
| Переменная | Описание |
|------------|----------|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Секрет для JWT токенов |
| `BOT_TOKEN` | Telegram Bot Token |

### Опциональные
| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `APP_PORT` | 8080 | Порт сервера |
| `MIN_BET` | 10 | Минимальная ставка |
| `MAX_BET` | 100000 | Максимальная ставка |
| `GAME_RATE_LIMIT` | 60 | Лимит игр в минуту |
| `GAME_RATE_WINDOW` | 60 | Окно лимита (сек) |
| `API_RATE_LIMIT` | 10 | Лимит API в минуту |
| `AUTH_RATE_LIMIT` | 5 | Лимит auth в минуту |
| `ADMIN_TELEGRAM_IDS` | - | ID админов через запятую |
| `ADMIN_BOT_ENABLED` | false | Включить админ бота |
| `LOG_FORMAT` | text | json для structured logs |
| `LOG_LEVEL` | info | debug, info, warn, error |
| `REDIS_URL` | - | Redis для rate limiting |
| `ALLOWED_ORIGIN` | - | CORS origin |
| `DEV_MODE` | - | Режим разработки |

---

## Статистика проекта

| Метрика | Значение |
|---------|----------|
| API Endpoints | 40+ |
| PvE Игры | 7 (CoinFlip, RPS, Mines, Case, Dice, Wheel, Mines Pro) |
| PvP Игры | 2 (RPS, Mines) |
| Режимов | 3 (PvE, PvP, Solo) |
| Валют | 2 (Gems, Coins) |
| Таблиц в БД | 10+ |
| Миграций | 13 |

---

## Лицензия

MIT
