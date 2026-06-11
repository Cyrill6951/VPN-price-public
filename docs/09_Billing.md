# VPN SaaS Enterprise

# Billing Service Specification

Version 1.0

---

# Общая концепция

Billing Service отвечает за:

• продажи

• подписки

• продление

• возвраты

• реферальные выплаты

• партнерские комиссии

• промокоды

• скидки

• налоги

• платежные шлюзы

• автоматические уведомления

• финансовые журналы

---

# Архитектура

                    Billing

                        │

──────────────────────────────────────

Order

Invoice

Payment

Subscription

Promo

Referral

Partner

Refund

Webhook

Ledger

Analytics

──────────────────────────────────────

                        │

                 PostgreSQL

                     Redis

                  ClickHouse

---

# Общий сценарий покупки

User

↓

Choose Plan

↓

Choose Country

↓

Choose Protocol

↓

Apply Promo

↓

Create Order

↓

Create Invoice

↓

Payment Gateway

↓

Webhook

↓

Payment Success

↓

Generate VPN

↓

Activate Subscription

↓

Send Notification

---

# Order

Order Status

NEW

WAIT_PAYMENT

PROCESSING

PAID

FAILED

EXPIRED

CANCELLED

REFUNDED

---

Order Fields

UUID

User

Plan

Country

Protocol

Devices

Promo

Amount

Currency

Status

Gateway

CreatedAt

UpdatedAt

---

# Invoice

Invoice Number

Amount

Gateway

Expire Time

Payment URL

Status

Metadata

---

# Payment

Payment ID

Gateway ID

External ID

Amount

Currency

Commission

Status

Raw Payload

Webhook Data

Created

Updated

---

Payment Status

CREATED

PENDING

SUCCESS

FAILED

EXPIRED

REFUNDED

PARTIAL_REFUND

---

# Subscription

Subscription ID

User

Server

Protocol

Country

Plan

ExpireAt

Status

Devices

Traffic

Speed

---

Subscription Status

ACTIVE

PAUSED

BLOCKED

EXPIRED

MIGRATED

DELETED

---

# Auto Renewal

Cron Daily

↓

Expire Soon

↓

Notify User

↓

Auto Charge

↓

Success

↓

Extend

↓

Notify

---

Failure

↓

Reminder

↓

Retry

↓

Cancel

---

# Grace Period

Expired

↓

3 Days

↓

VPN Active

↓

Reminder

↓

Payment

↓

Restore

---

# Payment Providers

Stripe

Telegram Payments

Telegram Stars

SBP

ЮKassa

CloudPayments

Robokassa

NowPayments

Cryptomus

Coinbase Commerce

PayPal (опционально)

---

# Crypto

BTC

ETH

USDT

TON

TRX

BNB

LTC

DOGE

---

# Card Payment

Create Invoice

↓

Redirect

↓

Bank

↓

Webhook

↓

Success

↓

Activate VPN

---

# Telegram Payment

Invoice

↓

Telegram

↓

Pay

↓

Webhook

↓

Activate

↓

VPN

---

# SBP

QR

↓

Bank App

↓

Webhook

↓

Subscription

---

# Manual Payment

Создать заявку

↓

Показать реквизиты

↓

Оплата

↓

Загрузка чека

↓

OCR

↓

Проверка

↓

Approve

↓

VPN

---

# OCR

PDF

PNG

JPEG

HEIC

↓

Extract

↓

Amount

↓

Date

↓

Card

↓

Reference

↓

Validation

---

# Refund

Request

↓

Validation

↓

Manager

↓

Approve

↓

Gateway

↓

Refund

↓

Log

---

Refund Status

NEW

WAIT

SUCCESS

FAILED

CANCELLED

---

# Promocode

FIX

PERCENT

FREE_MONTH

FREE_YEAR

PARTNER

FIRST_ORDER

REFERRAL

---

Validation

Date

Usage

User

Country

Plan

Limit

---

# Referral

Invite Link

↓

Registration

↓

Purchase

↓

Bonus

↓

Balance

↓

Withdraw

---

Commission

5%

10%

15%

20%

Custom

---

# Partner Billing

Partner

↓

Client

↓

Payment

↓

Commission

↓

Balance

↓

Withdrawal

---

# Withdrawal

Partner

↓

Request

↓

Approve

↓

Transfer

↓

History

---

# Ledger

Все финансовые операции

Immutable

Audit

Trace

Hash

Signature

---

Operation Types

Buy

Renew

Refund

Bonus

Referral

Commission

Manual

Correction

---

# Taxes

VAT

GST

Sales Tax

Country Tax

Custom Tax

---

# Anti Fraud

Velocity

Duplicate

Geo

Fingerprint

BIN

IP

Device

Blacklist

AI Score

---

Risk

LOW

MEDIUM

HIGH

BLOCK

---

# Retry Logic

Webhook Failed

↓

Retry 1 min

↓

Retry 5 min

↓

Retry 30 min

↓

Retry 1 hour

↓

Dead Queue

---

# Financial Reports

Revenue

Profit

Commission

Refund

Conversion

Retention

Countries

Protocols

Plans

Partners

---

# Accounting

Income

Expense

Balance

Payout

Tax

Fee

Profit

---

# Notification

Payment Success

Payment Failed

Refund

Subscription

Expire Soon

Partner Income

---

# API

POST /buy

POST /renew

POST /refund

POST /withdraw

POST /invoice

GET /payment

GET /history

GET /subscription

GET /plans

GET /promocode

---

# Webhooks

payment.success

payment.failed

refund.success

refund.failed

invoice.created

subscription.created

subscription.expired

partner.payout

---

# Queue

RabbitMQ

↓

Billing Worker

↓

Gateway

↓

Webhook

↓

DB

↓

Notify

---

# Security

HMAC

JWT

IP Whitelist

Signature

Replay Protection

Nonce

Timestamp

Audit Log

---

# SLA

99.99%

Multi Gateway

Auto Retry

Geo Redundancy

Backup Provider

Circuit Breaker

---

# KPI

MRR

ARR

ARPU

LTV

CAC

Churn

Conversion

Revenue

Profit

Retention

---

# White Label

Собственный биллинг

Собственные шлюзы

Собственные комиссии

Собственная валюта

Собственные налоги

Единая инфраструктура

---

# Итог

Полностью автоматизированный биллинг

Подписочная модель

Поддержка множества платежных систем

Автоматическая выдача VPN

Автоматическое продление

Партнерские выплаты

Реферальная система

Enterprise Ready

High Availability Ready