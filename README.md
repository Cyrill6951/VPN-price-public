# VPN SaaS Platform

Go + модульный монолит. Подробное видение — в [`docs/`](docs/), план реализации —
[`docs/21_Implementation_Plan.md`](docs/21_Implementation_Plan.md).

## Этап M0 — Фундамент

Скелет API-сервиса: конфиг из env, structured logging (slog), пулы Postgres/Redis,
эндпоинты `/health` `/ready` `/metrics`, graceful shutdown, первая миграция, Docker-стек, CI.

## Быстрый старт

```bash
cp .env.example .env
make up            # поднимает postgres, redis, minio, api (сборка в Docker)
make migrate-up    # применяет миграции
curl localhost:8080/health
curl localhost:8080/ready
curl localhost:8080/metrics
```

Локальный Go не требуется — сборка идёт в multi-stage Docker.

## Структура

```
cmd/api        главный бинарь (API Gateway + модули)
internal/
  platform/    общая инфраструктура (config, logger, db, cache, observability, httpx)
  ...          бизнес-модули (auth, user, billing, vpn, server, ...) — добавляются по плану
migrations/    goose SQL-миграции
deploy/        docker-compose, ansible, terraform
web/ admin/    Next.js фронтенд и CMS (добавляются на M5)
```

## Команды

`make help` — список доступных команд.
