# Mobile API

Version: 1.0
Status: спецификация (ранее — черновая заглушка)

> API для нативных мобильных приложений (iOS/Android) и Telegram Mini App.
> Базируется на общем ядре (`16_API.md`), но оптимизирован под мобильный клиент:
> компактные ответы, офлайн-кэш, push-токены, биометрия/passkey, обновление конфигов «на лету».

---

## 1. Базовый префикс и версии

```
/mobile/v1/
```
Те же принципы, что в `16_API.md`: JWT + Refresh, версионирование, обратная совместимость.

## 2. Особенности относительно основного API

- **Device binding:** каждый запрос несёт `X-Device-Id` и `X-Platform` (ios/android), привязка сессии к устройству.
- **Push-токены:** регистрация APNs/FCM токена для уведомлений (срок подписки, миграция, оплата).
- **Биометрия / Passkey (WebAuthn):** опциональный второй фактор и разблокировка приложения.
- **Компактные DTO:** ответы урезаны до полей, нужных мобильному UI; пагинация курсорная.
- **Дельта-синхронизация:** клиент кэширует список VPN/стран/тарифов и тянет только изменения по `updated_after`.

---

## 3. Аутентификация

| Метод | Endpoint | Назначение |
|---|---|---|
| POST | `/mobile/v1/auth/register` | регистрация (email/телефон/Telegram) |
| POST | `/mobile/v1/auth/login` | вход, возвращает access+refresh |
| POST | `/mobile/v1/auth/refresh` | обновление токена |
| POST | `/mobile/v1/auth/logout` | выход, инвалидация устройства |
| POST | `/mobile/v1/auth/passkey/register` | привязка passkey |
| POST | `/mobile/v1/auth/2fa/verify` | подтверждение TOTP/кода |

## 4. Устройства и push

| Метод | Endpoint | Назначение |
|---|---|---|
| POST | `/mobile/v1/devices/register` | регистрация устройства + push-токен |
| DELETE | `/mobile/v1/devices/{id}` | отвязать устройство |
| PUT | `/mobile/v1/devices/push-token` | обновить APNs/FCM токен |

## 5. VPN

| Метод | Endpoint | Назначение |
|---|---|---|
| GET | `/mobile/v1/vpn/list` | активные конфиги пользователя (компактно) |
| GET | `/mobile/v1/vpn/{id}/config` | конфиг (URI/JSON для системного импорта) |
| GET | `/mobile/v1/vpn/{id}/qr` | QR (PNG/SVG) |
| GET | `/mobile/v1/vpn/{id}/status` | статус/срок/сервер/нагрузка |
| POST | `/mobile/v1/vpn/{id}/rotate` | перегенерация ключа |
| GET | `/mobile/v1/servers/recommended` | лучший сервер по latency/geo для текущего клиента |

## 6. Каталог и покупка

| Метод | Endpoint | Назначение |
|---|---|---|
| GET | `/mobile/v1/catalog?updated_after=` | тарифы+страны+протоколы (дельта) |
| POST | `/mobile/v1/billing/buy` | создать заказ |
| POST | `/mobile/v1/billing/iap/verify` | **верификация in-app purchase (Apple/Google)** |
| POST | `/mobile/v1/billing/renew` | продление |
| GET | `/mobile/v1/billing/history` | история платежей |

> Примечание: для iOS/Android App Store обычно требуется поддержка **IAP** (StoreKit / Google Play Billing) — отдельная ветка верификации квитанций на сервере, параллельно вебхукам обычных провайдеров.

## 7. Реалтайм

- WebSocket `/mobile/v1/ws` — события: `vpn`, `payment`, `migration`, `notification`.
- Фоллбэк на push при отсутствии сокета.

## 8. Split Tunnel (клиентская часть)

- `GET/POST/DELETE /mobile/v1/routing/rules` — синхронизация правил (см. `Split Tunnel.md`).
- Проверки утечек: DNS/IPv6/WebRTC выполняются на клиенте, результат логируется на сервер.

## 9. Безопасность

- TLS 1.3, certificate pinning на клиенте, JWT + device binding, rate-limit (мобильный лимит), anti-replay (nonce+timestamp).

## 10. Связь с планом

Реализация — этап **M8** (после стабилизации основного API). IAP-верификация и push — обязательны до публикации в сторах.
