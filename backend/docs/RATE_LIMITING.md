# Rate limiting and migration notes

Текущее поведение:

- В коде реализован Redis-backed rate limiter: `internal/http/middleware/ratelimit_redis.go`.
- Глобальный лимит для `/api` группы: 10 запросов в минуту по IP (настраивается через env).
- Специальный более строгий лимит для `POST /api/auth`: 5 запросов в минуту по IP (настраивается через env).

Ограничения текущей реализации:

- Хранение состояний в процессе (Go `map`) — при рестарте или масштабировании (`multiple instances`) счётчики теряются.
- Потенциальный рост памяти при большом числе уникальных IP.
- Не подходит для production с несколькими инстансами.

Рекомендации по продакшн-миграции:

1. Использовать Redis (например, с алгоритмом fixed window или token bucket).

   - Redis поддерживает atomic INCR/EXPIRE и готовые реализации токенов.
   - Можно использовать библиотеку, например `go-redis/redis_rate` или `ulule/limiter`.

2. Централизовать конфигурацию лимитов и оставить более строгие лимиты для рискованных конечных точек
   (как `/api/auth`). Для вспомогательных конечных точек оставить более мягкие лимиты.

3. Добавить метрики (Prometheus) и алерты на превышение лимитов и рост числа заблокированных IP.

4. При необходимости — добавить временную блокировку (blacklist) для IP, демонстрирующих явную атаку.

Команды для быстрого развёртывания (пример с Redis + `ulule/limiter`):

1. Запустить локальный Redis (docker):

```bash
docker run -p 6379:6379 --name redis -d redis:7
```

2. Redis-конфигурация: `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`.

   - `InitRedisRateLimiter(addr, password, db)` проверяет подключение и при ошибке оставляет поведение fail-open.
   - Если Redis недоступен — лимитер пропускает запросы, но устанавливает заголовок `X-RateLimit-Error: redis-error` при ошибках.

Если хотите, могу: (а) подготовить дополнительные улучшения (авторотация ключей, blacklist, lua-скрипты для token-bucket), или (б) добавить интеграционные тесты CI, которые запускают Redis контейнер перед тестами.

Дополнительно в этом репозитории реализованы:

- Экспорт метрик Prometheus на `/metrics` (см. `cmd/app/main.go`).
- Метрики лимитера: `rate_limiter_requests_total{endpoint="..."}` и `rate_limiter_blocked_total{endpoint="..."}`.
- Параметры лимитов можно настраивать через env:
  - `API_RATE_LIMIT` (default 10)
  - `API_RATE_WINDOW_SECONDS` (default 60)
  - `AUTH_RATE_LIMIT` (default 5)
  - `AUTH_RATE_WINDOW_SECONDS` (default 60)

Пример локальной проверки:

```bash
docker run -p 6379:6379 --name redis -d redis:7
API_RATE_LIMIT=20 AUTH_RATE_LIMIT=5 go run ./cmd/app
curl http://localhost:8080/metrics | head -n 40
```
