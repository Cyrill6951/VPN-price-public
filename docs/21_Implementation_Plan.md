# VPN SaaS — Implementation Plan (Go / Modular Monolith)

Version: 1.0
Status: рабочий план реализации
Дата: 2026-06-11

> Этот документ «приземляет» концептуальное видение (файлы 01–20) в конкретный
> инженерный план. Файлы 01–20 описывают **финальную Global-платформу**
> (~30 микросервисов, 5000 серверов). Данный план описывает **путь от нуля до
> работающего продукта** через модульный монолит с последующим выделением сервисов.

---

## 0. Зафиксированные решения

| Решение | Выбор | Обоснование |
|---|---|---|
| Язык ядра | **Go 1.23+** | сетевой/SSH-оркестрационный слой, конкурентность, низкая латентность агентов |
| Архитектура старта | **Модульный монолит** | один деплой, чёткие модули; выделение в сервисы — по мере роста нагрузки |
| Frontend / Landing | **Next.js 15 + React 19 + Tailwind** | как в 06/20, лендинг генерируется через Lovable |
| Telegram | бот на Go (`gopkg.in/telebot.v4`) | единый язык с ядром |
| СУБД | PostgreSQL 17 (OLTP), Redis (cache/queue), ClickHouse (позже, аналитика) | как в 03/17 |
| Доступ к БД | `pgx` + `sqlc` (типобезопасные запросы) | без тяжёлого ORM |
| Миграции | **`goose`** | **заменяет Flyway** (Flyway — Java, не подходит для Go-стека) |
| Фоновые задачи | `asynq` (Redis) на старте → RabbitMQ при выделении сервисов | проще, чем RabbitMQ на MVP |
| VPN-протоколы MVP | **WireGuard** (`wgctrl`) + **Xray: VLESS+Reality** (gRPC API) | покрывают 90% спроса и обхода блокировок |
| Секреты | `.env` + Docker secrets на MVP → **Vault** на этапе Security | не блокировать старт Vault'ом |
| Деплой ноды | Ansible playbook по SSH | как в 11/18; Terraform — на этапе авто-скейлинга |
| CI/CD | GitHub Actions: lint → test → build → docker push → deploy | как в 18 |

---

## 1. Канонизация модели данных (устранение расхождений)

В репозитории две расходящиеся версии схемы: `03_Database.md` и `17_Database.md`.
**Канон для реализации — следующий гибрид** (всё остальное считать историческим):

- **Ключевое расхождение (VPN):** разделяем сущности, как в `17`:
  - `subscriptions` — бизнес-сущность (план, срок, статус, сервер, протокол, страна);
  - `vpn_configs` — сгенерированный конфиг (текст/URI/QR-ссылка, версия, hash, encrypted);
  - `vpn_keys` — криптоматериал (private/public/psk/uuid), связан с `vpn_configs`.
  Причина: ротация ключей и версионирование конфигов без потери истории подписки.
- Везде: `id UUID PK`, `created_at`, `updated_at`, `deleted_at` (soft delete), JSONB для `settings/metadata/payload`.
- `refunds` выносим в отдельную таблицу (как в `17`), а не статусом платежа.
- Партиционирование (как в обоих): `server_metrics`/`vpn_traffic` — по дням; `api_log`/`audit_log`/`analytics` — по месяцам.

Действие: на этапе **M2** написать единый `schema.sql` + goose-миграции; после этого
`03` и `17` помечаются «superseded by goose migrations».

Прочие неточности документации, учтённые в плане:
- `06_CMS.md` фактически описывает **Frontend/Dashboard**, а не CMS (CMS — это `07`). В коде: `web/` = клиентский кабинет+лендинг, `admin/` = CMS.
- `_MobileAPI.md` и `_ResellerAPI.md` были заглушками — заполнены (см. соответствующие файлы).

---

## 2. Структура репозитория (монорепо)

```
/
├── cmd/
│   ├── api/            # HTTP API Gateway + модули (главный бинарь)
│   ├── bot/            # Telegram-бот (может стартовать в том же процессе на MVP)
│   ├── worker/         # фоновые задачи (asynq)
│   └── agent/          # агент, устанавливаемый на VPN-ноду
├── internal/
│   ├── auth/           # регистрация, JWT, refresh, OAuth, 2FA
│   ├── user/           # профиль, устройства, настройки
│   ├── billing/        # orders, payments, subscriptions, promo, refunds
│   ├── vpn/            # config generator, key manager, выдача
│   ├── server/         # inventory, deploy, health, balancer, migration
│   ├── telegram/       # FSM, меню, оплата в боте
│   ├── notify/         # email/push/telegram/webhook
│   ├── monitoring/     # сбор метрик, health score, block detector
│   ├── support/        # тикеты
│   ├── partner/        # рефералы, партнёры, reseller
│   ├── cms/            # админ-эндпоинты
│   └── platform/       # общее: db, redis, config, logger, errors, rbac, audit
├── migrations/         # goose
├── api/                # OpenAPI 3.1 spec + сгенерированные типы
├── deploy/
│   ├── docker-compose.yml
│   ├── ansible/        # prepare/docker/vpn/monitor/security playbooks
│   └── terraform/      # (этап авто-скейлинга)
├── web/                # Next.js: лендинг + личный кабинет
├── admin/              # Next.js/React: CMS (или общий с web на route-группах)
└── docs/
```

Принцип: модуль `internal/X` экспортирует **интерфейс сервиса** (`type Service interface`)
и не лезет в чужие таблицы напрямую → когда придёт время, модуль вырезается в отдельный
бинарь без переписывания вызывающего кода.

---

## 3. Поэтапный план

Каждый этап = рабочий релиз с измеримым Definition of Done (DoD).
Длительности — ориентир для команды из 2 backend + 1 frontend + 1 DevOps.

### M0 — Фундамент (1–1.5 нед)
**Цель:** проект собирается, поднимается и деплоится.
- Монорепо, `cmd/api` со скелетом, `internal/platform` (config, structured logger, pgx-пул, Redis).
- `docker-compose`: postgres, redis, minio, app. `.env.example`.
- goose-миграция `V001_init` (extensions, базовые enum'ы).
- GitHub Actions: `lint (golangci-lint) → test → build → docker`.
- Healthchecks: `/health`, `/ready`, `/metrics` (Prometheus).
- **DoD:** `docker compose up` → API отвечает 200 на `/health`; CI зелёный.

### M1 — Auth & Users (1.5 нед) — *спринт 2*
- Регистрация (email+пароль), логин, JWT (access 15м / refresh 30д + ротация + blacklist в Redis).
- Telegram Login, парольная политика (12+, проверка утечек), профиль, устройства, RBAC (роли из `13`).
- **DoD:** полный login-flow + refresh + logout покрыты интеграционными тестами; RBAC-middleware работает.

### M2 — Каноническая БД + VPN Core (2.5 нед) — *спринт 3*
- Полный `schema.sql` по разделу 1 (users, subscriptions, vpn_configs, vpn_keys, servers, countries, plans, …) в goose-миграциях.
- **WireGuard**: генерация ключей (`wgctrl`), назначение IP, сборка `.conf`, QR (PNG/SVG), URI.
- **Xray VLESS+Reality**: генерация UUID/ShortID/keys, добавление инбаунда через Xray gRPC API.
- Хранение конфигов в MinIO (encrypted), выдача `GET /vpn/config`, `GET /vpn/qr`.
- Одна **реальная VPN-нода**, поднятая Ansible-плейбуком (docker: xray + wireguard + agent + node_exporter).
- **DoD:** через API создаётся рабочий WireGuard- и VLESS/Reality-конфиг, подключение реально работает с тестового устройства.

### M3 — Billing & Subscriptions (2.5 нед) — *спринт 4*
- Orders/Invoices/Payments/Subscriptions (FSM статусов из `09`), promo, ledger (immutable).
- **2 платёжных провайдера на старт:** Telegram Stars + один карточный/крипто (ЮKassa **или** Cryptomus).
- Вебхуки оплаты → событие `payment.success` → авто-генерация VPN → активация подписки → уведомление. Идемпотентность + ретраи (1/5/30 мин → DLQ).
- Grace period, авто-продление (cron), anti-fraud базовый (velocity, duplicate).
- **DoD:** оплата → VPN выдан и активен **< 30 сек** (главная цель из `02`), end-to-end тест на песочнице провайдера.

### M4 — Telegram Bot (2 нед) — *спринт 5*
- FSM (`08`): /start → меню → купить (страна/протокол/план/промокод) → оплата → выдача QR/URI.
- «Мои VPN», продление, рефералка, поддержка, FAQ, push-уведомления о сроке (за 7/3/1 день).
- **DoD:** полная покупка и продление VPN **внутри Telegram** без сайта.

### M5 — CMS & Frontend-кабинет (2.5 нед) — *спринты 6, частично 7*
- Личный кабинет (Next.js): dashboard, Мои VPN, подписки, платежи, устройства, рефералы, поддержка, настройки.
- CMS (`admin/`): users, servers, vpn, orders/payments, support, promo, news/FAQ, settings, audit log, RBAC.
- Подключение лендинга (Lovable, `20`) к реальным `GET /plans|/countries|/protocols`.
- **DoD:** админ управляет пользователями/серверами/платежами из панели; кабинет показывает реальные данные.

### M6 — Monitoring, Health & Migration (2.5 нед) — *спринты 7–8*
- Агент на ноде шлёт метрики каждые 10с → Redis + (ClickHouse) → Health Score (`10/11`).
- Prometheus + Grafana + Loki + AlertManager; Blackbox/Node exporter.
- **Block Detector** (Google/YouTube/Telegram/ChatGPT/Cloudflare).
- Балансировщик (least-conn/latency/geo) + **резервный пул** + **авто-миграция** клиентов при offline/блокировке (новый конфиг + уведомление).
- Drain mode, rolling update нод.
- **DoD:** падение ноды → клиенты автоматически переехали на резерв + получили уведомление; дашборды живые.

### M7 — Security hardening & Backups (1.5 нед) — *спринт 10*
- **Vault** (миграция секретов из .env), ротация ключей, шифрование чувствительных полей.
- 2FA (TOTP), audit log с hash-chain, WAF/rate-limit на gateway, fail2ban на нодах, Cloudflare.
- Бэкапы: БД (PITR, WAL→MinIO каждые 5 мин), конфиги, секреты. Проверка восстановления.
- DevSecOps: SAST, secret-scan, dependency-scan, container-scan в CI.
- **DoD:** пройден базовый чек-лист OWASP ASVS L1; восстановление из бэкапа протестировано.

### M8 — Partners, Referral, White Label, API (2.5 нед) — *спринт 11*
- Реферальная система (выплаты, баланс, вывод), партнёрский кабинет, комиссии.
- Reseller API + Mobile API (см. `_ResellerAPI.md`, `_MobileAPI.md`).
- White Label: бренды/домены/боты/тарифы поверх общей инфраструктуры (multi-tenant).
- Публичный OpenAPI 3.1 + Swagger UI + API-ключи с лимитами.
- **DoD:** партнёр продаёт под своим брендом; внешний клиент создаёт VPN по API-ключу.

### M9 — Production hardening (1.5 нед) — *спринт 12*
- Нагрузочное тестирование, оптимизация, авто-скейлинг (Terraform create VDS при load>80%).
- Pentest, Disaster Recovery учения, документация, прайс/тарифы, юридические страницы (ToS/Privacy/Refund).
- **DoD:** прохождение Acceptance Checklist из `18`; SLA-цели MVP (99% uptime, 1000 users).

---

## 4. Целевая карта эволюции монолит → микросервисы

Выделяем в отдельный сервис **по триггеру нагрузки/команды**, а не сразу:
1. `server/agent` — уже отдельный бинарь (на нодах).
2. `vpn` + `server` (VPN/Server Manager) — первыми, т.к. CPU/IO-интенсивные.
3. `billing` — из-за изоляции платёжных секретов и комплаенса.
4. `telegram`, `notify`, `monitoring/ai` — далее.
Связь между выделенными сервисами: gRPC + RabbitMQ (как в `05`).

---

## 5. Что НЕ входит в MVP (осознанно отложено)

- ClickHouse-аналитика (на старте метрики в Postgres/Redis), Kafka, Kubernetes/Helm.
- AI-предсказание отказов и AI-поддержка (`14`) — после накопления данных мониторинга.
- OpenVPN/VMESS/Trojan/Shadowsocks/SOCKS5 — добавляются после WireGuard+Reality.
- Multi-region/GeoDNS/Anycast, OCR ручных платежей, десятки платёжных провайдеров.
- Split Tunnel (отдельный файл) — требует клиентских приложений, этап после M8.

---

## 6. Критические риски

| Риск | Митигация |
|---|---|
| Юрисдикция/логи VPN, легальность платежей | проработать ToS/Privacy/«no-logs» и выбор юрисдикции **до** публичного запуска (M3/M9) |
| Блокировки Reality/доменов | резервный пул + авто-миграция (M6) обязательны до масштабирования |
| Идемпотентность платежей | строго на M3: уникальный ключ операции, ретраи, ledger |
| Безопасность SSH-оркестрации | ключи в Vault, ротация, отдельный bastion (M2/M7) |
| Расхождение схемы БД | устранено канонизацией (раздел 1) |

---

## 7. Ближайшие шаги (следующая сессия)

1. Инициализировать репозиторий и `go.mod`, поднять структуру из раздела 2.
2. `docker-compose.yml` + `.env.example` + миграция `V001_init`.
3. Скелет `cmd/api` с `/health`, `/ready`, `/metrics` и GitHub Actions.

> Готово к старту M0.
