# VPN SaaS Enterprise Platform

# System Architecture

Version: 1.0

---

# 1. Общая концепция

Платформа представляет собой распределенную SaaS-систему продажи VPN с полностью автоматизированным управлением инфраструктурой.

Основные цели:

- продажа VPN-подписок;
- централизованное управление серверами;
- автоматическое масштабирование;
- автоматическое восстановление после отказов;
- автоматическая миграция пользователей;
- единый API;
- единая CMS;
- Telegram Bot;
- White Label;
- Reseller Panel;
- Mobile API.

---

# 2. Архитектура

                          Internet

                              │

                ┌────────────────────────┐
                │ CDN + WAF + AntiDDoS   │
                └────────────────────────┘

                              │

                    Reverse Proxy Cluster

                      Nginx / Traefik

                              │

                ┌─────────────┴─────────────┐

                ▼                           ▼

        Landing Frontend             Personal Cabinet

          NextJS SSR                    React SPA

                │                           │

                └──────────────┬────────────┘

                               ▼

                    Backend API Gateway

                               │

      ┌────────────────────────────────────────────┐

      ▼          ▼          ▼          ▼

 Billing     Telegram    CMS API    Mobile API

      │          │          │          │

      └──────────┴──────────┴──────────┘

                     Core Services

        ┌──────────────────────────────────┐

        ▼

 VPN Management Service

        │

 ┌────────────┬────────────┬────────────┐

 ▼            ▼            ▼

 WireGuard   Xray      OpenVPN

              │

     ┌────────┴────────┐

     ▼                 ▼

  VLESS             VMESS

     ▼

 Shadowsocks

     ▼

 SOCKS Proxy

---

# 3. Infrastructure

Каждая страна представляет отдельный пул VDS.

Пример:

Germany-01
Germany-02
Germany-03

Reserve-Germany

USA-East
USA-West

Reserve-USA

Finland

France

Turkey

Singapore

Japan

UK

Canada

etc.

---

# 4. Central Management

Все VDS подключаются через SSH.

При регистрации автоматически:

- установка Docker;
- установка VPN;
- регистрация в CMS;
- регистрация в Monitoring;
- регистрация в Billing;
- регистрация в Balancer.

---

# 5. Auto Scaling

При нагрузке >80%

↓

создается новая VDS

↓

автоматическая установка

↓

включение в пул

↓

перераспределение клиентов

---

# 6. Auto Recovery

Server Offline

↓

Health Check

↓

Retry

↓

Reserve Server

↓

Create Config

↓

Update Client

↓

Push Telegram

↓

Email

↓

Webhook

---

# 7. Microservices

Auth Service

User Service

Billing Service

VPN Service

Server Manager

Telegram Service

Notification Service

Monitoring Service

Analytics Service

Partner Service

Promo Service

Support Service

CMS Service

---

# 8. Databases

PostgreSQL

Redis

ClickHouse

MinIO

Vault

---

# 9. Queue

RabbitMQ

или Kafka

---

# 10. Monitoring

Prometheus

Grafana

Loki

AlertManager

Blackbox Exporter

Node Exporter

---

# 11. Deployment

Docker

Docker Compose

Kubernetes

Helm

Terraform

Ansible

GitHub Actions

---

# 12. Security

TLS 1.3

JWT

OAuth2

Refresh Token

2FA

RBAC

Vault

Audit Log

IP Whitelist

Rate Limit

Fail2Ban

WAF

DDoS Protection

---

# 13. High Availability

N+1 Servers

Geo Redundancy

Automatic Failover

Automatic Config Migration

Automatic DNS Update

Automatic Balancing

99.99% SLA

---

# 14. White Label

Unlimited brands

Unlimited domains

Separate CMS

Separate Billing

Separate Telegram Bots

Separate Landing

Single Infrastructure

---

# 15. Reseller

Partner Cabinet

Own Tariffs

Own Branding

Own API

Own Statistics

Commission System

Referral System

---
