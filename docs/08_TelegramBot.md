# VPN SaaS Enterprise

# Telegram Bot Specification

Version 1.0

---

# Общая концепция

Telegram Bot является полноценным клиентом VPN-платформы.

Пользователь может полностью пользоваться сервисом без посещения сайта.

Все действия выполняются внутри Telegram.

Поддерживаются:

• Telegram Bot API

• Telegram WebApp

• Telegram Mini App

• Telegram Login

• Inline Mode

• Callback Query

• Deep Link

• Push Notification

---

# Архитектура

                    Telegram

                         │

                Telegram Bot API

                         │

──────────────────────────────────

Auth

VPN

Billing

Support

Referral

Promo

Notification

Analytics

──────────────────────────────────

                         │

                    Backend API

                         │

                 PostgreSQL

                    Redis

                    Vault

---

# Авторизация

/start

↓

Получение Telegram ID

↓

Проверка Whitelist

↓

Есть?

↓

ДА

↓

Создать сессию

↓

Главное меню

---

Нет

↓

Создать заявку

↓

Ожидание администратора

↓

Approve

или

Reject

---

# White List

Разрешение по:

Telegram ID

Username

Phone

Email

Invite Link

Promo Code

Partner Invite

---

# FSM (машина состояний)

START

↓

AUTH

↓

MENU

↓

BUY

↓

COUNTRY

↓

PROTOCOL

↓

PLAN

↓

PAYMENT

↓

WAIT

↓

CREATE VPN

↓

SEND CONFIG

↓

MENU

---

# Главное меню

🛒 Купить VPN

📄 Мои VPN

🌍 Страны

🔐 Протоколы

💳 Продлить

🎁 Промокод

👥 Рефералы

📞 Поддержка

⚙ Настройки

❓ FAQ

📰 Новости

---

# Купить VPN

Выбор тарифа

↓

Выбор страны

↓

Выбор протокола

↓

Количество устройств

↓

Промокод

↓

Оплата

↓

Создание VPN

↓

Отправка ключа

---

# Страны

🇩🇪 Германия

🇫🇮 Финляндия

🇫🇷 Франция

🇺🇸 США

🇨🇦 Канада

🇬🇧 Великобритания

🇯🇵 Япония

🇸🇬 Сингапур

🇹🇷 Турция

🇵🇱 Польша

и др.

Для каждой страны отображается:

Ping

Load

Online

Health Score

Свободные места

---

# Протоколы

WireGuard

VLESS

VMESS

Reality

Trojan

OpenVPN

Shadowsocks

SOCKS5

Для каждого:

Описание

Преимущества

Совместимость

Скорость

Рекомендации

---

# Выбор тарифа

1 день

7 дней

30 дней

90 дней

180 дней

365 дней

Lifetime (опционально)

---

# Оплата

Поддерживаются:

Telegram Stars

Банковская карта

СБП

Stripe

ЮKassa

CloudPayments

Robokassa

Cryptomus

NowPayments

Криптовалюта

---

# Сценарий оплаты

Создать Order

↓

Создать Invoice

↓

Оплатить

↓

Webhook

↓

Payment Success

↓

VPN Generate

↓

Send Config

---

# Выдача VPN

После оплаты бот отправляет:

QR-код

URI

CONF

JSON

ZIP

TXT

Инструкцию по подключению

---

# Раздел "Мои VPN"

Отображает:

Страна

Протокол

Дата окончания

Статус

Количество устройств

IP сервера

Кнопки:

Получить QR

Скачать

Удалить

Продлить

Перегенерировать

---

# Продление

Выбор подписки

↓

Выбор срока

↓

Оплата

↓

Продление

↓

Уведомление

---

# Автоматическое уведомление

За 7 дней

За 3 дня

За 1 день

В день окончания

После окончания

---

# Реферальная программа

Получить ссылку

↓

Пригласить друга

↓

Покупка

↓

Начисление %

↓

Баланс

↓

Вывод

---

# Промокоды

Ввести код

↓

Проверка

↓

Активен

↓

Применить

↓

Пересчет стоимости

---

# Новости

Новые серверы

Новые страны

Обновления

Акции

Скидки

Плановые работы

---

# FAQ

Что такое VPN

Как подключить

Почему не работает

Как оплатить

Как продлить

Как вернуть деньги

---

# Поддержка

Создать тикет

↓

Оператор

↓

Диалог

↓

Закрыть

Поддержка файлов:

PNG

JPG

PDF

TXT

LOG

---

# Настройки

Язык

Тема

Уведомления

Автопродление

Telegram Push

Email

SMS

---

# Push Notifications

Подписка заканчивается

VPN создан

Сервер заменен

Платеж успешен

Ответ поддержки

Новости

Акции

---

# Массовые рассылки

Только Admin

↓

Создать сообщение

↓

Выбрать аудиторию

↓

Отправить

↓

Статистика

---

# Inline Keyboard

Купить

Продлить

Получить QR

Получить CONF

Поддержка

FAQ

Настройки

---

# WebApp

Открывается внутри Telegram.

Поддерживает:

Dashboard

VPN

Payments

Devices

Referral

Support

Settings

Analytics

---

# Mini App

Полностью заменяет сайт.

SSR

SPA

PWA

Telegram Native

---

# Админ-панель бота

Пользователи

VPN

Платежи

Тикеты

Рассылки

Мониторинг

Серверы

Whitelist

Логи

---

# Whitelist

Добавить

Удалить

Импорт CSV

Импорт Excel

Telegram ID

Username

Phone

---

# Anti Spam

Rate Limit

Captcha

Cooldown

Flood Protection

Blacklist

---

# AI Assistant

Автоматические ответы

FAQ

Диагностика

Проверка сервера

Проверка подписки

Рекомендации

---

# API

POST /telegram/send

POST /telegram/menu

POST /telegram/vpn

POST /telegram/payment

POST /telegram/news

POST /telegram/broadcast

GET /telegram/user

GET /telegram/vpn

GET /telegram/history

---

# Webhook

telegram.message

telegram.payment

telegram.callback

telegram.inline

telegram.login

telegram.command

---

# Логирование

Все действия пользователя

Все команды

Все платежи

Все ошибки

Все подключения

Все миграции

---

# White Label

Несколько ботов

Разные токены

Разные бренды

Разные меню

Разные тарифы

Единая инфраструктура

---

# Итог

Полностью автономный Telegram-клиент

Без необходимости посещения сайта

Покупка VPN за 30 секунд

Автоматическая выдача конфигурации

Поддержка всех функций платформы

Enterprise Ready

MiniApp Ready

WebApp Ready

White Label Ready