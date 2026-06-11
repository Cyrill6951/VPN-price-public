# VPN SaaS Enterprise

# API Specification

Version 1.0

OpenAPI 3.1

REST + gRPC + WebSocket

---

# Общая концепция

API является единой точкой взаимодействия:

• Frontend

• CMS

• Telegram Bot

• Mobile App

• White Label

• Partner API

• AI Engine

• Monitoring

• Billing

• External Services

Все API имеют версионирование.

```
/api/v1/
/api/v2/
/admin/
/internal/
/partner/
/telegram/
/webhook/
```

---

# Authentication

Поддерживается:

JWT

Refresh Token

OAuth2

Telegram Login

API Key

HMAC Signature

Passkey

---

## Headers

Authorization

Bearer JWT

X-API-Key

X-Signature

X-Request-ID

X-Trace-ID

X-Timestamp

---

# User API

## Registration

POST

```
/api/v1/auth/register
```

Body

```
email
password
telegram_id
language
country
```

Response

```
user
token
refresh
```

---

## Login

POST

```
/api/v1/auth/login
```

---

## Refresh

POST

```
/api/v1/auth/refresh
```

---

## Logout

POST

```
/api/v1/auth/logout
```

---

## Profile

GET

```
/api/v1/users/me
```

---

PUT

```
/api/v1/users/me
```

---

DELETE

```
/api/v1/users/me
```

---

# Subscription API

GET

```
/subscriptions
```

POST

```
/subscriptions/create
```

PUT

```
/subscriptions/update
```

DELETE

```
/subscriptions/delete
```

POST

```
/subscriptions/renew
```

---

# VPN API

POST

```
/vpn/create
```

POST

```
/vpn/delete
```

POST

```
/vpn/migrate
```

POST

```
/vpn/rotate
```

GET

```
/vpn/list
```

GET

```
/vpn/config
```

GET

```
/vpn/qr
```

GET

```
/vpn/status
```

GET

```
/vpn/history
```

---

# Protocol API

GET

```
/protocols
```

WireGuard

Reality

VLESS

VMESS

OpenVPN

Trojan

Shadowsocks

SOCKS5

---

# Country API

GET

```
/countries
```

GET

```
/countries/load
```

GET

```
/countries/ping
```

GET

```
/countries/status
```

---

# Server API

GET

```
/servers
```

GET

```
/servers/health
```

GET

```
/servers/load
```

GET

```
/servers/metrics
```

POST

```
/servers/drain
```

POST

```
/servers/restart
```

POST

```
/servers/deploy
```

---

# Billing API

POST

```
/buy
```

POST

```
/renew
```

POST

```
/invoice
```

POST

```
/refund
```

GET

```
/payments
```

GET

```
/history
```

---

# Promo API

POST

```
/promo/check
```

POST

```
/promo/apply
```

GET

```
/promo/list
```

---

# Referral API

GET

```
/referral
```

GET

```
/referral/history
```

POST

```
/referral/create
```

POST

```
/referral/withdraw
```

---

# Partner API

GET

```
/partner/dashboard
```

GET

```
/partner/users
```

GET

```
/partner/orders
```

POST

```
/partner/create
```

POST

```
/partner/payout
```

---

# Telegram API

POST

```
/telegram/send
```

POST

```
/telegram/menu
```

POST

```
/telegram/payment
```

POST

```
/telegram/news
```

GET

```
/telegram/user
```

GET

```
/telegram/vpn
```

---

# CMS API

GET

```
/admin/users
```

GET

```
/admin/orders
```

GET

```
/admin/payments
```

GET

```
/admin/vpn
```

GET

```
/admin/servers
```

GET

```
/admin/logs
```

GET

```
/admin/settings
```

POST

```
/admin/deploy
```

POST

```
/admin/migrate
```

POST

```
/admin/news
```

---

# Monitoring API

GET

```
/metrics
```

GET

```
/health
```

GET

```
/alerts
```

GET

```
/forecast
```

GET

```
/geo
```

GET

```
/sla
```

---

# AI API

POST

```
/ai/support
```

POST

```
/ai/predict
```

POST

```
/ai/fraud
```

POST

```
/ai/recommend
```

GET

```
/ai/dashboard
```

GET

```
/ai/risk
```

---

# Notification API

POST

```
/notify
```

POST

```
/broadcast
```

GET

```
/notifications
```

---

# Support API

POST

```
/ticket
```

POST

```
/ticket/message
```

GET

```
/ticket/list
```

GET

```
/ticket/history
```

---

# File API

POST

```
/upload
```

GET

```
/download
```

DELETE

```
/delete
```

---

# Webhooks

payment.success

payment.failed

vpn.created

vpn.deleted

vpn.migrated

vpn.rotated

subscription.created

subscription.expired

partner.payout

ticket.created

server.offline

server.online

migration.started

migration.finished

deploy.finished

alert.created

---

# WebSocket

/ws

events:

vpn

payment

telegram

server

monitoring

support

analytics

notification

---

# gRPC Services

VPNService

BillingService

ServerService

MonitoringService

TelegramService

AnalyticsService

PartnerService

AIService

NotificationService

SupportService

---

# Rate Limits

Anonymous

60 req/min

User

600 req/min

Partner

5000 req/min

Internal

Unlimited

---

# Pagination

limit

offset

cursor

next

previous

---

# Filtering

country

protocol

status

provider

date

user

partner

---

# Sorting

asc

desc

created

updated

price

country

load

health

---

# Errors

200 OK

201 Created

400 Bad Request

401 Unauthorized

403 Forbidden

404 Not Found

409 Conflict

422 Validation

429 Too Many Requests

500 Internal Error

503 Maintenance

---

# OpenAPI

Swagger UI

Redoc

JSON

YAML

SDK Generation

---

# SDK

Go

NodeJS

Python

PHP

Java

C#

Rust

Swift

Kotlin

---

# Security

TLS1.3

JWT

RBAC

ABAC

RateLimit

Vault

Audit

Replay Protection

Nonce

HMAC

---

# Versioning

v1

v2

v3

Backward Compatible

Deprecation Policy

---

# White Label

Отдельные API Keys

Отдельные Limits

Отдельные Domains

Отдельные Webhooks

Общая инфраструктура

---

# Итог

REST Ready

gRPC Ready

WebSocket Ready

OpenAPI Ready

SDK Ready

Enterprise Ready

Cloud Native Ready