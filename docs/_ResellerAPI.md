# Reseller API

Version: 1.0
Status: спецификация (ранее — черновая заглушка)

> API для реселлеров и White-Label-партнёров: продажа VPN под собственным брендом
> на общей инфраструктуре. Дополняет `09_Billing.md` (Partner Billing) и `01_Architecture.md` (Reseller).

---

## 1. Модель

```
PARTNER (reseller)
   │  api_key, commission, balance, brand
   ▼
CLIENTS (конечные пользователи реселлера)
   │
   ▼
SUBSCRIPTIONS / VPN_CONFIGS  (на общей инфраструктуре)
```

- Реселлер не управляет серверами — он управляет **клиентами и тарифами**; инфраструктура общая (multi-tenant).
- Изоляция данных по `partner_id` (row-level), отдельные API-ключи, лимиты и вебхуки.

## 2. Аутентификация

```
Authorization: Bearer <partner_jwt>      # партнёрский кабинет
X-API-Key: <reseller_api_key>            # программный доступ
X-Signature: HMAC(body, secret)          # подпись запросов
```
Префикс: `/partner/v1/` и `/reseller/v1/`.

## 3. Управление клиентами

| Метод | Endpoint | Назначение |
|---|---|---|
| POST | `/reseller/v1/clients` | создать клиента |
| GET | `/reseller/v1/clients` | список (фильтры, пагинация) |
| GET | `/reseller/v1/clients/{id}` | карточка клиента |
| PUT | `/reseller/v1/clients/{id}` | обновить (статус, лимиты) |
| DELETE | `/reseller/v1/clients/{id}` | заблокировать/удалить |

## 4. Подписки и выдача VPN

| Метод | Endpoint | Назначение |
|---|---|---|
| POST | `/reseller/v1/subscriptions` | создать подписку клиенту (план/страна/протокол) |
| POST | `/reseller/v1/subscriptions/{id}/renew` | продлить |
| POST | `/reseller/v1/subscriptions/{id}/suspend` | приостановить |
| GET | `/reseller/v1/subscriptions/{id}/config` | получить конфиг/URI/QR |
| POST | `/reseller/v1/subscriptions/{id}/rotate` | ротация ключа |

> Выдача VPN идёт через то же ядро (`internal/vpn`), просто помечается `partner_id`.
> Списание — с **баланса реселлера** (предоплата) либо по постоплатной комиссии.

## 5. Тарифы реселлера

| Метод | Endpoint | Назначение |
|---|---|---|
| GET | `/reseller/v1/plans` | свои тарифы (наценка поверх себестоимости) |
| POST | `/reseller/v1/plans` | создать тариф |
| PUT | `/reseller/v1/plans/{id}` | изменить цену/лимиты |

## 6. Финансы

| Метод | Endpoint | Назначение |
|---|---|---|
| GET | `/reseller/v1/balance` | текущий баланс |
| POST | `/reseller/v1/balance/topup` | пополнение |
| GET | `/reseller/v1/transactions` | движения средств (ledger) |
| POST | `/reseller/v1/payout` | запрос вывода комиссии |
| GET | `/reseller/v1/stats` | продажи, активные подписки, churn, выручка |

## 7. White Label (бренд реселлера)

| Метод | Endpoint | Назначение |
|---|---|---|
| GET/PUT | `/reseller/v1/brand` | логотип, цвета, домен, SMTP |
| GET/PUT | `/reseller/v1/bot` | токен и меню собственного Telegram-бота |
| GET/PUT | `/reseller/v1/landing` | блоки лендинга (Landing Builder из `07`) |

## 8. Вебхуки реселлера

```
client.created
subscription.created
subscription.expired
payment.success
payout.processed
```
Доставка с ретраями (1/5/30 мин → DLQ), подпись HMAC.

## 9. Лимиты и безопасность

- Rate limit: `Partner 5000 req/min` (как в `16_API.md`).
- Изоляция tenant'а, аудит всех действий, IP allow-list, replay protection.

## 10. Связь с планом

Реализация — этап **M8** (Partners / White Label). Требует готовых `internal/billing` (M3) и `internal/vpn` (M2) с поддержкой `partner_id` (заложить в схему на M2).
