# VPN SaaS Enterprise

# Business Logic

Version 1.0

---

# Общая логика платформы

Платформа предназначена для автоматизированной продажи VPN-доступов с минимальным участием администратора.

Все процессы должны работать автоматически.

Основная цель — пользователь получает VPN менее чем за 30 секунд после оплаты.

---

# Участники системы

Guest

User

VIP User

Partner

Reseller

Support

Moderator

Administrator

SuperAdmin

Developer

System

AI Monitoring

---

# User Journey

Посещение сайта

↓

Выбор тарифа

↓

Выбор страны

↓

Выбор протокола

↓

Регистрация

↓

Оплата

↓

Создание VPN

↓

Получение ключа

↓

Подключение

↓

Продление

↓

История платежей

---

# Guest Flow

Открывает Landing

↓

Просматривает тарифы

↓

Просматривает страны

↓

Просматривает преимущества

↓

Нажимает Купить

↓

Регистрация

---

# Registration

Email

или

Telegram Login

или

Google OAuth

или

Apple ID

или

GitHub

или

Phone

↓

Подтверждение

↓

Создание аккаунта

↓

JWT

↓

Refresh Token

↓

Вход

---

# Telegram Registration

Старт бота

↓

Получение Telegram ID

↓

Проверка Whitelist

↓

Разрешить

или

Запретить

↓

Создать аккаунт

↓

Привязать UserID

---

# Покупка VPN

Выбор тарифа

↓

Выбор страны

↓

Выбор протокола

↓

Выбор количества устройств

↓

Переход к оплате

↓

Создание заказа

↓

Ожидание оплаты

↓

Оплата

↓

Подтверждение

↓

Создание VPN

↓

Выдача конфигурации

↓

Push уведомление

---

# Продление

Выбор подписки

↓

Продлить

↓

Выбор периода

↓

Оплата

↓

Обновление даты окончания

↓

Push

---

# Автоматическое продление

За 3 дня

↓

Напоминание

↓

Автооплата (если разрешена)

↓

Продление

↓

Push

---

# Личный кабинет

Мои VPN

↓

Мои устройства

↓

История

↓

Платежи

↓

Промокоды

↓

Рефералы

↓

Поддержка

↓

Настройки

---

# Добавление устройства

Пользователь

↓

Получает новый ключ

↓

Добавляется DeviceID

↓

Синхронизация

↓

Статистика

---

# Ограничение устройств

Если превышен лимит

↓

Запрет создания

↓

Предложить Upgrade

---

# Выбор страны

Получение списка

↓

Показ нагрузки

↓

Показ Ping

↓

Выбор

↓

Создание конфигурации

---

# Выбор протокола

WireGuard

VLESS

VMESS

OpenVPN

Shadowsocks

SOCKS5

↓

Создание конфигурации

---

# Генерация конфигурации

Создание UUID

↓

Создание ключей

↓

Добавление пользователя

↓

Регистрация на сервере

↓

Сохранение

↓

Выдача

---

# QR

После генерации

↓

QR PNG

↓

Telegram

↓

WEB

↓

Mobile

---

# API выдачи

GET /config

↓

JWT

↓

Получение

↓

Download

---

# Проверка подписки

Cron

Каждый час

↓

Expired?

↓

Да

↓

Удаление доступа

↓

Уведомление

---

# Проверка серверов

Каждые 10 секунд

↓

Ping

↓

CPU

↓

RAM

↓

Disk

↓

Loss

↓

Status

↓

Health Score

---

# Health Score

100

↓

Excellent

80

↓

Good

60

↓

Warning

40

↓

Critical

20

↓

Migration

0

↓

Offline

---

# Миграция клиентов

Server Offline

↓

Reserve Found

↓

Generate Config

↓

DB Update

↓

Telegram Push

↓

Email

↓

Webhook

↓

Client Download

---

# Автоматическая балансировка

Least Connection

или

Round Robin

или

Geo

или

Latency

↓

Move Client

---

# Мониторинг блокировок

Google

YouTube

Telegram

Netflix

Discord

ChatGPT

Spotify

Cloudflare

↓

Blocked?

↓

Yes

↓

AI Score

↓

Reserve Server

↓

Migration

---

# Создание тикета

User

↓

Support

↓

Operator

↓

Reply

↓

Close

---

# Telegram Bot

/start

↓

Авторизация

↓

Главное меню

↓

Купить VPN

↓

Оплата

↓

Получить ключ

↓

Продлить

↓

Поддержка

↓

FAQ

---

# White Label

Выбор бренда

↓

Своя CMS

↓

Свой Landing

↓

Свой Telegram

↓

Свой API

↓

Свой Billing

---

# Partner

Создание клиента

↓

Получение %

↓

Баланс

↓

Вывод

↓

История

---

# Promo

Промокод

↓

Проверка

↓

Активен?

↓

Да

↓

Применить скидку

↓

Создать Order

---

# Refund

Создание заявки

↓

Проверка

↓

Approval

↓

Возврат

↓

Закрытие

---

# AI Monitoring

Сбор метрик

↓

ML Анализ

↓

Предсказание отказа

↓

Предложение миграции

↓

Автоматическое действие

---

# Массовая миграция

Страна заблокирована

↓

Создать новый пул

↓

Перегенерация

↓

Рассылка

↓

Переключение

↓

Удаление старых

---

# Backup

Каждый день

↓

Config

↓

Database

↓

Secrets

↓

MinIO

↓

Archive

---

# Disaster Recovery

DataCenter OFF

↓

Reserve Region

↓

Restore

↓

DNS Update

↓

Traffic Move

↓

Online

---

# Полный жизненный цикл пользователя

Landing

↓

Register

↓

Buy

↓

Payment

↓

VPN Create

↓

Download

↓

Connect

↓

Use

↓

Renew

↓

Referral

↓

Support

↓

Upgrade

↓

Expire

↓

Archive

