# VPN SaaS Enterprise

# Frontend Architecture

Version 1.0

---

# Общая архитектура

Frontend построен на:

NextJS 15+

React 19+

TypeScript

TailwindCSS

TanStack Query

Redux Toolkit

Zustand

Framer Motion

PWA

SSR

ISR

CSR

---

# Общая структура

src/

app/

components/

widgets/

entities/

features/

shared/

hooks/

layouts/

services/

api/

store/

styles/

types/

utils/

assets/

public/

middleware.ts

---

# Design System

Radius 16px

Glassmorphism

Dark Theme

Light Theme

Blur

Gradient

Animation

Responsive

Accessibility

WCAG AA

---

# Цвета

Primary

Blue

Secondary

Purple

Success

Green

Warning

Orange

Danger

Red

Background

#0F172A

Card

#1E293B

Text

#FFFFFF

Border

#334155

---

# Layout

Header

Hero

Sidebar

Content

Footer

Modal

Drawer

Toast

Dialog

Loading

Skeleton

---

# Landing

/

Главный экран

Hero

CTA

Advantages

Countries

Protocols

Pricing

Reviews

FAQ

Footer

---

# Hero

Заголовок

VPN нового поколения

Без ограничений

Без логов

100+ серверов

Подключение за 30 секунд

Кнопка Купить

Кнопка Telegram

---

# Блок преимуществ

Высокая скорость

WireGuard

VLESS

Reality

OpenVPN

Shadowsocks

SOCKS5

Автопереключение

Неограниченный трафик

---

# Карта

Интерактивная карта

Маркер сервера

Нагрузка

Ping

Статус

Свободные места

---

# Страница тарифов

/plans

Карточки

Цена

Описание

Количество устройств

Кнопка Купить

---

# Checkout

/checkout

Тариф

Страна

Протокол

Количество устройств

Промокод

Оплата

Подтверждение

---

# Dashboard

/dashboard

Sidebar

Статистика

VPN

Подписки

Платежи

Устройства

Рефералы

Поддержка

Настройки

---

# Мои VPN

/dashboard/vpn

Название

Страна

Протокол

Дата окончания

Кнопка QR

Скачать

Удалить

Перегенерировать

---

# Генерация конфигурации

Получить

WireGuard

↓

QR

↓

Conf

↓

Download

↓

Telegram

↓

Email

---

# QR

PNG

SVG

Copy URI

Download

Print

---

# Подписки

/dashboard/subscriptions

Активные

История

Продлить

Upgrade

Downgrade

---

# История платежей

/dashboard/payments

Дата

Сумма

Метод

Статус

Invoice

PDF

---

# Устройства

/dashboard/devices

Windows

Linux

Android

iPhone

Mac

Router

TV

Удалить

Переименовать

---

# Реферальная программа

/dashboard/referral

Ссылка

QR

Доход

Друзья

Баланс

Вывод

---

# Поддержка

/dashboard/support

Создать тикет

История

Чат

Файлы

Закрыть

---

# Настройки

/dashboard/settings

Email

Telegram

Язык

Тема

Уведомления

2FA

API Key

---

# API Keys

Создать

Удалить

Скопировать

Permissions

---

# Административная CMS

/admin

---

Dashboard

Users

Servers

VPN

Orders

Payments

Support

Telegram

Monitoring

Analytics

Promocodes

Partners

Logs

Settings

---

# Dashboard CMS

Количество пользователей

Онлайн

Доход

Активные VPN

Нагрузка серверов

CPU

RAM

Ping

Loss

---

# Users

Поиск

Редактирование

Удаление

Блокировка

Смена тарифа

Продление

---

# Servers

Добавить

Удалить

SSH

Load

CPU

RAM

Ping

Clients

Reserve

Restart

---

# Monitoring

Grafana Widget

Ping

Loss

CPU

Traffic

Blocked

Health Score

---

# VPN Configs

WireGuard

Reality

VLESS

VMESS

OpenVPN

Shadowsocks

SOCKS5

Создать

Удалить

Продлить

---

# Telegram

Whitelist

Рассылка

Новости

Статистика

Поддержка

---

# Billing

Stripe

SBP

ЮKassa

Crypto

CloudPayments

Robokassa

---

# Partners

Баланс

Продажи

Комиссия

API

White Label

---

# Analytics

MRR

ARR

LTV

ARPU

Funnels

Countries

Servers

Protocols

Revenue

---

# Mobile

Responsive

360px

768px

1024px

1440px

1920px

---

# PWA

Offline

Install

Push

Background Sync

Share

---

# SEO

Meta

OG

Twitter

JSON-LD

Schema

Sitemap

Robots

Canonical

Hreflang

---

# Performance

Lazy

Dynamic Import

Image Optimize

ISR

Edge Cache

CDN

Compression

---

# Security

CSP

XSS

CSRF

Secure Cookie

HttpOnly

SameSite

Fingerprint

RateLimit

---

# Lighthouse

Performance 100

SEO 100

Accessibility 100

Best Practices 100

PWA 100

---

# Итог

SSR

SPA

PWA

Responsive

Enterprise UI

White Label Ready

Telegram Ready

Mobile Ready

SEO Ready