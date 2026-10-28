# Replider E2E — Demo Video Recording

Playwright-based browser tests for the Replider dashboard. Primarily used to record demo videos and capture app store screenshots using realistic seed data.

## Prerequisites

- Node.js 22+
- Flutter SDK (for building the web app)
- Docker (for running the API)

## Setup

```bash
cd e2e
npm install
npx playwright install chromium
```

## Recording the Demo Video

### 1. Start the API

```bash
cd api && make docker-dev
```

### 2. Build Flutter Web

```bash
cd app && flutter build web \
  --dart-define=API_BASE_URL=http://localhost:8080/v1
```

### 3. Seed Demo Data

```bash
cd e2e && npm run seed
```

This populates the database with a realistic dental clinic scenario:

| Resource | Details |
|----------|---------|
| Account | Clínica Saúde+ Odontologia |
| Admin | `demo@replider.com` / `demo1234` |
| AI Agent | "Assistente Saúde+" (Google Gemini) |
| Conversations | 10 realistic patient conversations (35 messages) |
| Knowledge Base | 5 FAQ chunks (hours, insurance, procedures, emergencies) |
| Channel | Telegram "WhatsApp Clínica" |

### 4. Record the Video

```bash
npm run test:demo
```

Playwright auto-starts a static server for the Flutter app on port 3000 — no manual serve step needed.

The video is saved to `e2e/test-results/` as a `.webm` file.

## What the Demo Captures

The recorded flow runs approximately 60–90 seconds:

1. **Login** — `demo@replider.com` / `demo1234`
2. **Dashboard** — conversation list with 10 realistic patient threads
3. **Open conversation** — chat history showing bot ↔ patient exchange
4. **Settings → Agent** — configured AI agent with system prompt
5. **Knowledge Base** — FAQ chunks indexed for RAG
6. **Analytics** — usage statistics dashboard
7. **Billing** — current plan and usage
8. **Back to Dashboard**

## Other Commands

```bash
npm test                    # Run all tests
npm run test:headed         # Run with visible browser window
npm run test -- --project=mobile  # Mobile viewport (iPhone 14, 390×844)
npx playwright show-report  # Open HTML report
```

## Output

| Path | Contents |
|------|---------|
| `test-results/` | Videos (.webm), screenshots, traces |
| `playwright-report/` | HTML test report |

Screenshots are captured automatically on every test run and organized by test name. For mobile screenshots, use `--project=mobile`.
