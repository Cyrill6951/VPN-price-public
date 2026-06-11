# VPN SaaS Enterprise

# Backend Architecture

Version 1.0

---

# Общая концепция

Backend представляет собой полностью микросервисную архитектуру.

Каждый сервис разворачивается независимо.

Связь между сервисами:

REST

gRPC

RabbitMQ

Kafka

Internal Events

---

# Общая схема

                    API Gateway

                         │

──────────────────────────────────────────

Auth

User

Billing

VPN

Server

Telegram

CMS

Analytics

Support

Partner

Notification

AI

Storage

Scheduler

Worker

──────────────────────────────────────────

                         │

                  PostgreSQL

                     Redis

                  ClickHouse

                    MinIO

                    Vault

---

# API Gateway

Единая точка входа.

Отвечает за:

JWT

Rate Limit

CORS

Logging

Proxy

Load Balance

Versioning

Geo Routing

Compression

TLS

---

Endpoints

/api/v1

/api/v2

/admin

/internal

/mobile

/bot

/webhook

---

Gateway middleware

Logger

JWT

RateLimit

GeoIP

DeviceID

Fingerprint

RBAC

Audit

---

# Auth Service

Регистрация

Авторизация

Refresh Token

Logout

2FA

Email Verify

Telegram Verify

OAuth

Password Reset

Session

---

REST

POST /register

POST /login

POST /refresh

POST /logout

POST /verify

POST /forgot

POST /reset

---

JWT

Access 15 min

Refresh 30 days

Rotation

Blacklist

---

# User Service

CRUD пользователей

Профиль

Настройки

Язык

Валюта

Страна

Устройства

История

---

GET /me

PUT /me

GET /devices

DELETE /device

GET /history

---

# VPN Service

Основной сервис платформы.

Создает:

WireGuard

VLESS

VMESS

OpenVPN

Shadowsocks

SOCKS5

Reality

Trojan

---

POST /vpn/create

POST /vpn/delete

POST /vpn/update

GET /vpn/config

GET /vpn/qr

GET /vpn/status

---

VPN Service работает через RPC

↓

Server Manager

↓

SSH

↓

Docker API

↓

Xray

↓

WireGuard

↓

OpenVPN

---

# Config Generator

Создает

conf

json

yaml

uri

png qr

zip

txt

---

Пример:

User

↓

UUID

↓

Keys

↓

Config

↓

QR

↓

Storage

↓

Send

---

# Server Manager

Подключение VDS

Мониторинг

Установка

Обновление

Перезагрузка

Удаление

---

Server Register

↓

SSH

↓

Install Docker

↓

Install VPN

↓

Firewall

↓

Monitoring

↓

Done

---

REST

POST /server/add

POST /server/remove

POST /server/update

GET /server/status

GET /server/load

GET /server/health

---

# Billing Service

Создает заказ

Принимает оплату

Подписки

Продление

Возврат

Комиссии

---

POST /buy

POST /renew

POST /refund

GET /invoice

GET /history

---

Поддержка

Stripe

Telegram

Crypto

SBP

CloudPayments

ЮKassa

Robokassa

---

# Notification Service

Telegram

Email

SMS

Push

Webhook

---

POST /notify

POST /broadcast

GET /history

---

RabbitMQ Consumer

↓

Queue

↓

Provider

↓

Delivery

↓

Status

---

# Telegram Service

Bot API

Webhook

Menu

Payment

QR

Support

Referral

Promo

VPN

---

Commands

/start

/help

/buy

/myvpn

/renew

/support

/referral

/promocode

/settings

---

# Monitoring Service

Health

Ping

CPU

RAM

Disk

Network

Docker

Xray

WG

OVPN

---

Каждые 10 секунд

↓

Metric

↓

Redis

↓

ClickHouse

↓

Grafana

---

# Analytics Service

LTV

ARPU

MRR

ARR

Revenue

Retention

Funnels

Geo

Speed

Usage

Load

---

ClickHouse

↓

Dashboard

↓

CMS

---

# Promo Service

Promo

Coupon

Referral

Gift

Bonus

---

POST /promo

GET /promo

DELETE /promo

---

# Referral Service

Invite

Track

Bonus

History

Withdraw

---

# Support Service

Tickets

Chat

Files

Priority

Operator

History

---

# CMS Service

Admin Panel

CRUD

News

FAQ

Promo

Users

Servers

Monitoring

Billing

Logs

---

# AI Service

Получает

Ping

Loss

Speed

Errors

CPU

RAM

Geo

Block

↓

ML

↓

Score

↓

Recommendation

↓

Migration

---

# Scheduler

Cron

Cleanup

Backup

Expire

Reminder

Health

Update

Deploy

---

Каждый час

Каждый день

Каждую неделю

Каждый месяц

---

# Worker Pool

Background Jobs

Mail

Push

Telegram

Webhook

Backup

Migration

Deploy

AI

---

100 workers

Scale

Auto

---

# Storage Service

MinIO

S3

QR

Config

Logs

Backup

Avatar

Images

---

# Secrets Service

Vault

SSH

Private Keys

JWT

Billing

API

Telegram

Cloud

---

# Cache

Redis

JWT

Session

Geo

Country

Config

Status

RateLimit

---

TTL

30 sec

1 min

5 min

1 hour

24 hour

---

# Event Bus

RabbitMQ

или

Kafka

---

Events

PaymentSuccess

SubscriptionCreated

VPNGenerated

ServerOffline

MigrationStarted

MigrationFinished

TicketCreated

UserRegistered

ReferralPaid

---

# Webhook Service

POST

payment.success

vpn.created

vpn.expired

server.offline

server.online

migration.done

ticket.created

---

Retry

Backoff

Dead Letter Queue

---

# Auto Scaling

Load > 80%

↓

Terraform

↓

Create VDS

↓

Install

↓

Register

↓

Balancer

↓

Ready

---

# Failover

Offline

↓

Reserve

↓

Move Clients

↓

Generate Config

↓

Notify

↓

Delete Old

---

# Internal RPC

gRPC

Server Manager

VPN

Billing

AI

Storage

Monitoring

---

# Logging

JSON

Correlation ID

Request ID

Trace ID

OpenTelemetry

---

# Health API

GET /health

GET /ready

GET /metrics

GET /live

---

# Version

v1

v2

v3

Backward Compatible

---

# Horizontal Scaling

API

Worker

VPN

Telegram

Billing

Notification

Monitoring

Analytics

Support

CMS

Scale Independently

---

# Итог

~30 микросервисов

1000+ REST Endpoint

200+ Internal Events

100+ Background Jobs

Cloud Native

Stateless

Kubernetes Ready

HA Ready

Geo Ready

Enterprise Ready