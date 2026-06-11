# VPN SaaS Enterprise

# Database Architecture

Version 1.0

PostgreSQL 17

Redis 8

ClickHouse

MinIO

Vault

---

# Общая концепция

Система хранения данных должна обеспечивать:

• отказоустойчивость

• горизонтальное масштабирование

• высокую скорость

• ACID

• репликацию

• шардирование

• аудит

• хранение больших объемов логов

• хранение VPN-конфигураций

• хранение аналитики

---

# Архитектура

                    Backend

                        │

────────────────────────────────────

PostgreSQL

Redis

ClickHouse

MinIO

Vault

────────────────────────────────────

                        │

Analytics

Billing

VPN

Telegram

CMS

Monitoring

AI

---

# PostgreSQL

Основная OLTP база.

Используется для:

Users

Orders

Payments

VPN

Servers

Subscriptions

CMS

Support

Referral

Partners

Audit

---

# Redis

Используется как:

Session Storage

JWT Cache

Geo Cache

Rate Limit

Locks

Queue

Config Cache

Metrics Cache

---

# ClickHouse

Используется для:

Analytics

Metrics

Traffic

Monitoring

Logs

Billing Analytics

Geo Analytics

AI Features

---

# MinIO

Используется для хранения:

QR

Configs

ZIP

Images

Backups

Logs

Invoices

Tickets

---

# Vault

Хранит:

JWT Secret

SSH Keys

TLS Keys

VPN Keys

Billing Keys

Telegram Token

Cloud API

DNS API

---

# USERS

uuid

email

phone

telegram_id

password_hash

language

country

timezone

status

created_at

updated_at

deleted_at

---

# USER_PROFILE

user_id

firstname

lastname

avatar

settings

preferences

---

# USER_SESSIONS

id

user_id

jwt

refresh

ip

device

fingerprint

expires_at

---

# DEVICES

id

user_id

name

os

version

last_seen

status

---

# SUBSCRIPTIONS

id

user_id

plan_id

vpn_id

server_id

protocol

country

expires_at

status

created_at

---

# VPN_CONFIGS

id

user_id

protocol

config

qr

version

hash

encrypted

created_at

---

# VPN_KEYS

id

vpn_id

private_key

public_key

psk

uuid

status

---

# SERVERS

id

provider

hostname

country

region

ipv4

ipv6

cpu

ram

disk

traffic

health

status

---

# SERVER_METRICS

server_id

cpu

ram

disk

traffic

connections

loss

ping

timestamp

---

# SERVER_HEALTH

server_id

score

blocked

vpn

docker

agent

updated

---

# ORDERS

id

user_id

amount

currency

gateway

status

created

updated

---

# PAYMENTS

id

order_id

gateway

external_id

amount

commission

status

payload

created

---

# REFUNDS

id

payment_id

amount

reason

status

created

---

# PROMOCODES

id

code

type

discount

usage

limit

expires

---

# REFERRALS

id

user_id

friend

bonus

status

created

---

# PARTNERS

id

name

email

commission

status

created

---

# PARTNER_USERS

partner_id

user_id

created

---

# SUPPORT_TICKETS

id

user_id

priority

status

subject

created

---

# SUPPORT_MESSAGES

ticket_id

user_id

message

file

created

---

# NEWS

id

title

text

lang

created

---

# FAQ

id

question

answer

lang

sort

---

# BANNERS

id

image

link

enabled

---

# SETTINGS

key

value

type

updated

---

# TELEGRAM_USERS

telegram_id

user_id

username

chat_id

language

status

---

# TELEGRAM_LOG

telegram_id

command

payload

created

---

# API_KEYS

id

user_id

key

permissions

expires

---

# API_LOG

request

response

ip

user

latency

created

---

# AUDIT_LOG

actor

action

object

payload

ip

created

---

# AI_EVENTS

server

score

prediction

action

created

---

# MIGRATIONS

user

old_server

new_server

reason

created

---

# DEPLOY_LOG

server

version

status

created

---

# NOTIFICATIONS

user

type

status

payload

created

---

# EMAIL_QUEUE

payload

status

created

---

# PUSH_QUEUE

payload

status

created

---

# SMS_QUEUE

payload

status

created

---

# WEBHOOKS

url

secret

status

created

---

# WEBHOOK_LOG

event

status

payload

created

---

# ANALYTICS

date

users

orders

payments

revenue

country

protocol

---

# VPN_TRAFFIC

vpn

rx

tx

speed

timestamp

---

# GEO_STAT

country

users

traffic

speed

latency

---

# DNS_LOG

server

domain

ip

status

created

---

# STORAGE_FILES

path

size

mime

hash

bucket

created

---

# BACKUPS

type

size

location

status

created

---

# Индексы

users(email)

users(telegram_id)

subscriptions(user_id)

vpn_configs(user_id)

payments(order_id)

servers(country)

servers(status)

metrics(server_id,timestamp)

audit(created)

api_log(created)

---

# JSONB

settings

preferences

payload

metadata

vpn_config

webhook

audit

logs

---

# Партиционирование

API_LOG

по месяцам

VPN_TRAFFIC

по дням

SERVER_METRICS

по дням

ANALYTICS

по месяцам

AUDIT_LOG

по месяцам

---

# Репликация

Primary

↓

Replica-1

↓

Replica-2

↓

Backup

↓

Cold Storage

---

# PITR

WAL Archive

5 минут

↓

Restore

Any Point

---

# Connection Pool

PgBouncer

Transaction Mode

Pool Size 500

---

# ClickHouse Tables

metrics

traffic

analytics

billing

geo

ai

logs

events

---

# Redis TTL

Session

24h

JWT

15m

RateLimit

1m

Geo

24h

Metrics

30s

---

# MinIO Buckets

configs

qr

backup

logs

avatars

tickets

billing

images

---

# Flyway

V001_init

V002_users

V003_servers

V004_vpn

V005_billing

V006_ai

V007_monitoring

...

---

# Capacity

100K users

↓

1 TB

1M users

↓

10 TB

10M users

↓

100 TB

Horizontal Scaling

---

# Итог

60+ таблиц

300+ индексов

JSONB

UUID

Partitioning

Replication

PITR

Redis Cache

ClickHouse Analytics

MinIO Storage

Cloud Native

Enterprise Ready