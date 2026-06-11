# 32. Split Tunnel (Smart Routing)

## Назначение

Пользователь может самостоятельно выбирать,
какой трафик должен идти через VPN,
а какой — напрямую через локального интернет-провайдера.

Это позволяет использовать VPN только для выбранных приложений или сайтов,
оставляя остальные соединения без туннелирования.

---

## Поддерживаемые режимы

• By App
• By Domain
• By IP
• By CIDR
• By Process
• By Port

---

## Режимы работы

Full Tunnel

Весь трафик идет через VPN.

---

Split Tunnel

Через VPN идет только выбранный трафик.

---

Inverse Split Tunnel

Через VPN идет весь трафик,
кроме явно исключенных ресурсов.

---

## Настройка

CMS:

Создать правило

↓

Указать:

Домен

или

IP

или

CIDR

или

Приложение

↓

Выбрать действие

VPN

или

Direct

↓

Сохранить

---

## Клиентские приложения

Windows

macOS

Linux

Android

iOS (с учетом ограничений платформы)

Router

---

## Политики

Route By Domain

Route By IP

Route By Application

Route By Process

Route By Country

Route By ASN

---

## API

POST /routing/rules

GET /routing/rules

DELETE /routing/rules

POST /routing/reload

---

## Мониторинг

Количество правил

Активные маршруты

Ошибки маршрутизации

DNS Status

Latency

Packet Loss

---

## Безопасность

Проверка DNS Leak

Проверка IPv6 Leak

Проверка WebRTC Leak

Audit Log

Policy Validation

---

## White Label

Индивидуальные политики маршрутизации
для каждого партнера.

---

Enterprise Ready