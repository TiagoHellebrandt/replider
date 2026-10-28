<div align="center">

# 🤖 Replider API

**Omnichannel AI Agent platform — Backend API**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![Swagger](https://img.shields.io/badge/OpenAPI-3.1-85EA2D?logo=swagger&logoColor=black)](http://localhost:8080/swagger/index.html)

</div>

---

## 📖 Overview

Replider is an **omnichannel AI Agent platform** that empowers businesses to deploy intelligent, autonomous agents across multiple communication channels (WhatsApp, web chat, and more). The API handles everything from real-time AI-powered conversations to billing, knowledge base management (RAG), and multi-channel webhook orchestration.

### Key Features

- 🧠 **AI-Powered Agents** — Multi-provider support (Gemini, OpenAI) with streaming responses
- 💬 **Real-Time Chat** — WebSocket-based communication with token streaming
- 📚 **Knowledge Base (RAG)** — Vector-powered retrieval-augmented generation via pgvector
- 📲 **Omnichannel Webhooks** — Inbound/outbound messaging across WhatsApp and other channels
- 💳 **Stripe Billing** — Subscription management with usage-based credit tracking
- 🔐 **Authentication** — JWT + Social Auth (Google, Meta) + Password Recovery
- 📅 **Google Calendar** — Agent skill for scheduling integration
- 📁 **Media Storage** — Google Cloud Storage for file uploads

---

## 🏗️ Architecture

The project follows **Clean Architecture** (Hexagonal / Ports & Adapters) principles:

```
cmd/
└── api/
    └── main.go              # Entry point, dependency injection, graceful shutdown

internal/
├── config/                  # Environment configuration loader
├── core/
│   ├── domain/              # Entities, interfaces (ports), business rules
│   ├── usecases/            # Application use cases (orchestrate domain logic)
│   └── services/            # Domain services (AI factory, etc.)
└── adapters/
    ├── http/
    │   ├── handlers/        # HTTP/WS handlers (controllers)
    │   ├── middleware/       # Auth, rate limiting
    │   └── router/          # Route registration
    ├── repository/postgres/ # Database access (sqlc-generated)
    ├── auth/                # JWT service implementation
    ├── ai/                  # AI provider adapters
    ├── payment/             # Stripe adapter
    ├── storage/             # GCS adapter
    ├── email/               # SMTP adapter
    ├── calendar/            # Google Calendar adapter
    └── channels/whatsapp/   # WhatsApp adapter

db/
├── migrations/              # Sequential SQL migrations (golang-migrate)
└── queries/                 # SQL queries (sqlc source of truth)
```

---

## ⚙️ Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Docker](https://docs.docker.com/get-docker/) | 20+ | Containerization |
| [Docker Compose](https://docs.docker.com/compose/) | v2+ | Service orchestration |
| [Go](https://go.dev/dl/) | 1.25+ | Local development (optional) |
| [golang-migrate](https://github.com/golang-migrate/migrate) | 4.x | Database migrations |
| [swag](https://github.com/swaggo/swag) | v2 | Swagger docs generation (optional) |

---

## 🚀 Getting Started

### 1. Clone & Configure

```bash
git clone <repo-url>
cd replider/api

# Create your environment file from the template
cp .env.example .env
```

Edit `.env` and fill in the required values. See [Environment Variables](#-environment-variables) for details.

### 2. Start the Development Environment

```bash
# Start API + PostgreSQL with hot reload (recommended)
make docker-dev
```

This uses [Air](https://github.com/air-verse/air) for **automatic hot reload** — every time you save a `.go` file, the API recompiles and restarts inside the container without manual intervention.

### 3. Run Migrations

In a separate terminal:

```bash
make migrate-up
```

### 4. Verify

```bash
# Health check
curl http://localhost:8080/health

# Swagger UI
open http://localhost:8080/swagger/index.html
```

---

## 📋 Available Commands

| Command | Description |
|---------|-------------|
| `make docker-dev` | 🔥 Start dev environment with hot reload (Air + Docker) |
| `make dev` | Run API locally (no Docker, requires local Postgres) |
| `make build` | Compile production binary to `bin/api` |
| `make migrate-up` | Apply all pending database migrations |
| `make migrate-down` | Rollback the last migration |
| `make migrate-create name=<name>` | Create a new migration file pair |
| `make swagger` | Generate Swagger 2.0 docs (`docs/`) |
| `make swagger-v3` | Generate OpenAPI 3.1 docs (`docs/`) |

### Docker Compose Profiles

| File | Purpose |
|------|---------|
| `docker-compose.dev.yml` | Development — hot reload, source mounted as volume |
| `docker-compose.yml` | Local testing — multi-stage production build |

---

## 🌐 API Endpoints

All routes are prefixed with `/v1` unless noted otherwise.

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check (DB connectivity) |
| `GET` | `/swagger/*` | Swagger UI |
| `POST` | `/v1/accounts` | Create a new account |
| `POST` | `/v1/auth/login` | Authenticate (email + password) |
| `POST` | `/v1/auth/forgot-password` | Request password reset email |
| `POST` | `/v1/auth/reset-password` | Reset password via token |
| `POST` | `/v1/auth/google` | Google OAuth login |
| `POST` | `/v1/auth/meta` | Meta OAuth login |
| `GET` | `/v1/plans` | List available subscription plans |
| `POST` | `/v1/payments/webhook` | Stripe webhook receiver |
| `POST` | `/v1/webhooks/:channel_token` | Inbound channel webhook (WhatsApp, etc.) |

### Protected (Bearer Token required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/me` | Get current user + account info |
| `GET` | `/v1/accounts/usage` | Get usage metrics for current billing cycle |
| `POST` | `/v1/agents` | Create an AI agent |
| `GET` | `/v1/agents` | List all agents |
| `PUT` | `/v1/agents/:id` | Update an agent |
| `PATCH` | `/v1/agents/:id/toggle` | Activate/deactivate an agent |
| `POST` | `/v1/agents/preview` | Preview chat with an agent |
| `POST` | `/v1/knowledge` | Ingest content into knowledge base (RAG) |
| `GET` | `/v1/knowledge` | List knowledge base chunks |
| `GET` | `/v1/ws/chat` | WebSocket — real-time AI chat (streaming) |
| `POST` | `/v1/payments/checkout` | Create a Stripe checkout session |
| `GET` | `/v1/conversations` | List conversations |
| `POST` | `/v1/conversations/:id/reply` | Send a human reply |
| `PATCH` | `/v1/conversations/:id/resume-bot` | Resume AI bot on a conversation |
| `POST` | `/v1/media/upload` | Upload a file to GCS |

> Full interactive docs available at **[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)** when the server is running.

---

## 🔑 Environment Variables

<details>
<summary>Click to expand</summary>

| Variable | Required | Description |
|----------|----------|-------------|
| **Server** | | |
| `PORT` | ✅ | Server port (default: `8080`) |
| `JWT_SECRET` | ✅ | Secret key for JWT signing |
| `APP_BASE_URL` | ✅ | Base URL for emails/links |
| **Database** | | |
| `DB_HOST` | ✅ | PostgreSQL host |
| `DB_PORT` | ✅ | PostgreSQL port |
| `DB_USER` | ✅ | PostgreSQL user |
| `DB_PASSWORD` | ✅ | PostgreSQL password |
| `DB_NAME` | ✅ | PostgreSQL database name |
| `DB_SSLMODE` | ✅ | SSL mode (`disable` for local dev) |
| **AI** | | |
| `AI_PROVIDER` | ✅ | Default AI provider (`gemini` or `openai`) |
| `GEMINI_API_KEY` | ⚡ | Required if `AI_PROVIDER=gemini` |
| `OPENAI_API_KEY` | ⚡ | Required if `AI_PROVIDER=openai` |
| **Stripe** | | |
| `STRIPE_SECRET_KEY` | ⚡ | Stripe API key |
| `STRIPE_WEBHOOK_SECRET` | ⚡ | Stripe webhook signing secret |
| **Storage** | | |
| `GCS_BUCKET_NAME` | ⚡ | Google Cloud Storage bucket |
| `GCS_CREDENTIALS_BASE64` | ⚡ | Base64-encoded GCS service account JSON |
| **Email** | | |
| `SMTP_HOST` | ⚡ | SMTP server host |
| `SMTP_PORT` | ⚡ | SMTP server port |
| `SMTP_USERNAME` | ⚡ | SMTP username |
| `SMTP_PASSWORD` | ⚡ | SMTP password |
| `SMTP_FROM` | ⚡ | Sender email address |
| **Calendar** | | |
| `GOOGLE_CALENDAR_CREDENTIALS_BASE64` | ⚡ | Base64-encoded Google Calendar credentials |

> ✅ = Always required &nbsp;&nbsp; ⚡ = Required for specific features

</details>

---

## 🗄️ Database

### Tech Stack

- **PostgreSQL 16** with **pgvector** extension for vector similarity search
- **[sqlc](https://sqlc.dev/)** for type-safe, generated Go code from SQL queries
- **[golang-migrate](https://github.com/golang-migrate/migrate)** for versioned migrations

### Migration Workflow

```bash
# Create a new migration
make migrate-create name=add_new_table

# Apply all migrations
make migrate-up

# Rollback the last migration
make migrate-down
```

Migration files are in `db/migrations/` following the `NNNNNN_description.{up,down}.sql` convention.

### Regenerating sqlc

After editing queries in `db/queries/`, regenerate the Go code:

```bash
sqlc generate
```

---

## 📚 API Documentation (Swagger)

The API is documented with [Swaggo](https://github.com/swaggo/swag) annotations.

```bash
# Generate Swagger 2.0
make swagger

# Generate OpenAPI 3.1
make swagger-v3
```

Install the swag CLI if not already available:

```bash
go install github.com/swaggo/swag/v2/cmd/swag@latest
```

Access the interactive Swagger UI at:
**[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)**

---

## 🧪 Running Tests

```bash
go test ./...
```

---

## 🐳 Production Deployment

The production `Dockerfile` uses a multi-stage build for a minimal, secure image:

```bash
# Build production image
docker compose up --build

# Or build directly
docker build -t replider-api .
```

The final image:
- Based on `alpine:3.19` (~5MB base)
- Runs as non-root user (`replider`)
- Includes CA certificates and timezone data
- Produces a single statically-linked binary

---

## 🛠️ Tech Stack

| Category | Technology |
|----------|-----------|
| Language | Go 1.25 |
| Framework | Gin |
| Database | PostgreSQL 16 + pgvector |
| ORM/Query | sqlc (type-safe SQL) |
| Migrations | golang-migrate |
| Auth | JWT (golang-jwt) + bcrypt |
| WebSocket | gorilla/websocket |
| AI | Google Gemini, OpenAI |
| Payments | Stripe |
| Storage | Google Cloud Storage |
| Email | SMTP |
| Calendar | Google Calendar API |
| Docs | Swaggo (OpenAPI 3.1) |
| Dev Tools | Air (hot reload), Docker Compose |

---

## 📄 License

MIT — see [LICENSE](../LICENSE) at the repository root.
