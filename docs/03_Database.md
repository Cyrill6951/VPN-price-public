# VPN SaaS Enterprise

# Database Design

Version 1.0

---

# Общая архитектура

СУБД:
PostgreSQL 17+

Кэш:
Redis

Аналитика:
ClickHouse

Объектное хранилище:
MinIO

Secrets:
Vault

---

# Основные требования

• ACID

• Горизонтальное масштабирование

• Репликация

• Point-In-Time Recovery

• WAL Archiving

• UUID вместо SERIAL

• Soft Delete

• Audit Log

• CreatedAt

• UpdatedAt

---

# USERS

Таблица пользователей

users

id UUID PK

email

phone

telegram_id

password_hash

role

status

language

timezone

currency

country

created_at

updated_at

deleted_at

INDEX:

email

telegram_id

phone

---

# USER_PROFILE

user_profiles

id

user_id

first_name

last_name

avatar

birth_date

city

country

settings_json

---

# USER_SESSIONS

user_sessions

id

user_id

jwt

refresh_token

ip

device

expires

created_at

---

# DEVICES

devices

id

user_id

device_uuid

platform

os

version

last_online

created_at

---

# PLANS

plans

id

name

price

currency

days

max_devices

traffic_limit

speed_limit

active

---

# ORDERS

orders

id

user_id

plan_id

status

amount

currency

payment_method

promo_id

created_at

paid_at

---

# PAYMENTS

payments

id

order_id

gateway

external_id

status

amount

fee

currency

payload

created_at

---

# SUBSCRIPTIONS

subscriptions

id

user_id

plan_id

server_id

protocol_id

status

expire_at

created_at

---

# VPN_PROTOCOLS

vpn_protocols

id

name

enabled

priority

---

WireGuard

VLESS

VMESS

OpenVPN

Shadowsocks

SOCKS5

Reality

Trojan

---

# VPN_KEYS

vpn_keys

id

subscription_id

public_key

private_key

config

uri

qr

status

created_at

---

# COUNTRIES

countries

id

iso

name

flag

enabled

priority

---

# SERVERS

servers

id

country_id

provider

hostname

ipv4

ipv6

ssh_port

api_port

status

reserve

priority

created_at

---

# SERVER_METRICS

server_metrics

id

server_id

cpu

ram

disk

traffic

loss

jitter

ping

clients

created_at

---

# SERVER_HEALTH

server_health

id

server_id

score

status

reason

created_at

---

# SERVER_POOL

server_pool

id

country

primary

secondary

reserve

enabled

---

# MIGRATIONS

migrations

id

user_id

from_server

to_server

reason

status

created_at

---

# CONFIG_HISTORY

config_history

id

subscription_id

old_config

new_config

reason

created_at

---

# NOTIFICATIONS

notifications

id

user_id

type

title

body

read

created_at

---

# SUPPORT_TICKETS

support_tickets

id

user_id

status

priority

subject

created_at

closed_at

---

# SUPPORT_MESSAGES

support_messages

id

ticket_id

sender

message

created_at

---

# PROMOCODES

promocodes

id

code

discount

type

expire_at

limit

used

---

# REFERRALS

referrals

id

parent_user

child_user

bonus

created_at

---

# REFERRAL_PAYMENTS

referral_payments

id

referral_id

amount

status

created_at

---

# PARTNERS

partners

id

company

api_key

balance

status

created_at

---

# PARTNER_USERS

partner_users

id

partner_id

user_id

created_at

---

# PARTNER_ORDERS

partner_orders

id

partner_id

order_id

commission

created_at

---

# TELEGRAM_USERS

telegram_users

id

telegram_id

user_id

username

language

created_at

---

# TELEGRAM_LOG

telegram_log

id

telegram_id

request

response

created_at

---

# API_KEYS

api_keys

id

user_id

key

expire_at

permissions

created_at

---

# API_LOG

api_log

id

api_key

method

path

ip

status

created_at

---

# AUDIT_LOG

audit_log

id

actor

action

object

payload

created_at

---

# BILLING_LOG

billing_log

id

payment

gateway

status

payload

created_at

---

# WEBHOOKS

webhooks

id

url

secret

status

created_at

---

# WEBHOOK_LOG

webhook_log

id

webhook

payload

response

created_at

---

# EMAIL_QUEUE

email_queue

id

email

template

status

created_at

---

# PUSH_QUEUE

push_queue

id

user_id

payload

status

created_at

---

# SMS_QUEUE

sms_queue

id

phone

payload

status

created_at

---

# AI_EVENTS

ai_events

id

server

prediction

confidence

action

created_at

---

# BLOCK_CHECK

block_check

id

server

google

youtube

telegram

chatgpt

cloudflare

status

created_at

---

# SERVER_IMAGES

server_images

id

provider

image

version

created_at

---

# SERVER_DEPLOY

server_deploy

id

server

version

status

log

created_at

---

# SERVER_BACKUPS

server_backups

id

server

archive

size

created_at

---

# STORAGE_FILES

storage_files

id

bucket

path

size

hash

created_at

---

# CMS_ADMINS

cms_admins

id

login

password

role

last_login

---

# CMS_LOG

cms_log

id

admin

action

payload

created_at

---

# SETTINGS

settings

id

key

value

updated_at

---

# FEATURES

features

id

feature

enabled

updated_at

---

# NEWS

news

id

title

body

published

created_at

---

# FAQ

faq

id

question

answer

sort

---

# BANNERS

banners

id

title

image

link

enabled

---

# LANDING_VISITS

landing_visits

id

ip

country

utm

device

created_at

---

# ANALYTICS

analytics

id

user

event

payload

created_at

---

# CLICKHOUSE

Используется:

логи

метрики

подключения

скорости

ошибки

нагрузка

API

Telegram

VPN

---

# Redis

Кэш:

JWT

Refresh

Session

RateLimit

HealthCheck

ServerStatus

DNS

---

# Vault

SSH Keys

Private Keys

JWT Secret

Billing Secret

Telegram Token

OpenRouter Token

Cloud API

WireGuard Keys

Reality Keys

---

ИТОГО

≈ 58 основных таблиц

≈ 300 индексов

≈ 120 внешних ключей

Поддержка 1+ млн пользователей

Поддержка 1000+ VDS

Поддержка 100000+ VPN конфигураций одновременно