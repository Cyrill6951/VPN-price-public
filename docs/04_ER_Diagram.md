# VPN SaaS Enterprise

# Entity Relationship Diagram (ER)

Version 1.0

---

# Общая схема

                     USERS
                        │
         ┌──────────────┼───────────────┐
         │              │               │
         ▼              ▼               ▼

 USER_PROFILE   USER_SESSIONS      DEVICES

         │
         ▼

 SUBSCRIPTIONS

         │

 ┌───────┴──────────────────────────┐

 ▼                                  ▼

 VPN_KEYS                      VPN_PROTOCOLS

         │

         ▼

 SERVERS

         │

         ▼

 COUNTRIES

---

# Платежная система

USERS

 │

 ▼

ORDERS

 │

 ▼

PAYMENTS

 │

 ▼

BILLING_LOG

---

# Telegram

USERS

 │

 ▼

TELEGRAM_USERS

 │

 ▼

TELEGRAM_LOG

---

# Support

USERS

 │

 ▼

SUPPORT_TICKETS

 │

 ▼

SUPPORT_MESSAGES

---

# Referral

USERS

 │

 ▼

REFERRALS

 │

 ▼

REFERRAL_PAYMENTS

---

# Partner

PARTNERS

 │

 ▼

PARTNER_USERS

 │

 ▼

PARTNER_ORDERS

---

# Server Cluster

COUNTRIES

 │

 ▼

SERVER_POOL

 │

 ▼

SERVERS

 │

 ├──────────────┐

 ▼              ▼

SERVER_HEALTH   SERVER_METRICS

 │

 ▼

BLOCK_CHECK

 │

 ▼

AI_EVENTS

 │

 ▼

MIGRATIONS

---

# CMS

CMS_ADMINS

 │

 ▼

CMS_LOG

 │

 ▼

AUDIT_LOG

---

# Notifications

USERS

 │

 ▼

NOTIFICATIONS

 │

 ├─────────────┐

 ▼             ▼

EMAIL_QUEUE  PUSH_QUEUE

 │

 ▼

SMS_QUEUE

---

# Storage

STORAGE_FILES

 │

 ▼

SERVER_BACKUPS

 │

 ▼

SERVER_DEPLOY

---

# API

API_KEYS

 │

 ▼

API_LOG

---

# Landing

LANDING_VISITS

 │

 ▼

ANALYTICS

---

# White Label

BRANDS

 │

 ▼

DOMAINS

 │

 ▼

LANDINGS

 │

 ▼

CMS

 │

 ▼

BILLING

 │

 ▼

TELEGRAM_BOT

---

# Reseller

PARTNERS

 │

 ▼

CLIENTS

 │

 ▼

SUBSCRIPTIONS

---

# Полные связи

USERS 1:N SUBSCRIPTIONS

USERS 1:N ORDERS

USERS 1:N PAYMENTS

USERS 1:N VPN_KEYS

USERS 1:N DEVICES

USERS 1:N NOTIFICATIONS

USERS 1:N SUPPORT_TICKETS

USERS 1:N USER_SESSIONS

USERS 1:N API_KEYS

USERS 1:N TELEGRAM_USERS

USERS 1:N REFERRALS

USERS 1:N PARTNER_USERS

---

SUBSCRIPTIONS 1:N VPN_KEYS

SUBSCRIPTIONS 1:N CONFIG_HISTORY

SUBSCRIPTIONS N:1 SERVERS

SUBSCRIPTIONS N:1 VPN_PROTOCOLS

---

SERVERS N:1 COUNTRIES

SERVERS 1:N SERVER_METRICS

SERVERS 1:N SERVER_HEALTH

SERVERS 1:N SERVER_BACKUPS

SERVERS 1:N SERVER_DEPLOY

SERVERS 1:N AI_EVENTS

SERVERS 1:N BLOCK_CHECK

SERVERS 1:N MIGRATIONS

---

ORDERS 1:1 PAYMENTS

PAYMENTS 1:N BILLING_LOG

---

PARTNERS 1:N PARTNER_USERS

PARTNERS 1:N PARTNER_ORDERS

---

SUPPORT_TICKETS 1:N SUPPORT_MESSAGES

---

PROMOCODES 1:N ORDERS

---

WEBHOOKS 1:N WEBHOOK_LOG

---

NEWS

FAQ

BANNERS

FEATURES

SETTINGS

используются CMS независимо.

---

# Масштабирование

Одна БД:

100 000 пользователей

↓

Master Replica

↓

500 000 пользователей

↓

Sharding

↓

5 000 000 пользователей

↓

Geo Cluster

↓

50 000 000 пользователей

---

# Partition

API_LOG

по месяцам

SERVER_METRICS

по дням

ANALYTICS

по месяцам

VPN_LOGS

по дням

LANDING_VISITS

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

Каждые 5 минут WAL

↓

Archive

↓

MinIO

↓

Restore Any Time

---

# Итоговая модель

58+ таблиц

120+ FK

300+ Index

JSONB

UUID

Partition

Replication

HA

Geo HA

Cloud Native