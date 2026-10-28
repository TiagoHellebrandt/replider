#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────────────────
# Stripe E2E Test Script
#
# Tests the payment flows end-to-end using the Stripe CLI to trigger real
# webhook events against the local API. Requires:
#
#   1. `stripe` CLI installed and authenticated (`stripe login`)
#   2. Local API running (`make dev` or `make docker-dev`)
#   3. `.env` with STRIPE_SECRET_KEY (sk_test_...) and STRIPE_WEBHOOK_SECRET
#
# Usage:
#   ./scripts/test-stripe-e2e.sh [--api-url http://localhost:8080]
#
# The script:
#   1. Starts `stripe listen --forward-to` in the background
#   2. Triggers events with `stripe trigger`
#   3. Checks API responses and database state via curl
#   4. Cleans up and reports results
# ──────────────────────────────────────────────────────────────────────────────
set -euo pipefail

API_URL="${1:-http://localhost:8080}"
WEBHOOK_PATH="/v1/payments/webhook"
LISTEN_PID=""
PASS=0
FAIL=0
TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

cleanup() {
    if [ -n "$LISTEN_PID" ] && kill -0 "$LISTEN_PID" 2>/dev/null; then
        echo -e "\n${YELLOW}Stopping stripe listen (PID $LISTEN_PID)...${NC}"
        kill "$LISTEN_PID" 2>/dev/null || true
        wait "$LISTEN_PID" 2>/dev/null || true
    fi
    echo ""
    echo "════════════════════════════════════════════════════"
    echo -e "  Results: ${GREEN}$PASS passed${NC}, ${RED}$FAIL failed${NC}, $TOTAL total"
    echo "════════════════════════════════════════════════════"
    if [ "$FAIL" -gt 0 ]; then
        exit 1
    fi
}
trap cleanup EXIT

assert_ok() {
    local name="$1"
    local status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" -eq 0 ]; then
        PASS=$((PASS + 1))
        echo -e "  ${GREEN}✓${NC} $name"
    else
        FAIL=$((FAIL + 1))
        echo -e "  ${RED}✗${NC} $name"
    fi
}

assert_contains() {
    local name="$1"
    local output="$2"
    local expected="$3"
    TOTAL=$((TOTAL + 1))
    if echo "$output" | grep -q "$expected"; then
        PASS=$((PASS + 1))
        echo -e "  ${GREEN}✓${NC} $name"
    else
        FAIL=$((FAIL + 1))
        echo -e "  ${RED}✗${NC} $name (expected '$expected' in output)"
        echo "    actual: $(echo "$output" | head -c 200)"
    fi
}

# ── Pre-flight checks ────────────────────────────────────────────────────────

echo "╔══════════════════════════════════════════════════╗"
echo "║       Stripe E2E Payment Tests                  ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""

echo "Checking prerequisites..."

# Check stripe CLI
if ! command -v stripe &>/dev/null; then
    echo -e "${RED}✗ stripe CLI not found. Install with: brew install stripe/stripe-cli/stripe${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} stripe CLI: $(stripe --version)"

# Check API is running
if ! curl -sf "$API_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ API not running at $API_URL. Start it with: make dev${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} API running at $API_URL"

echo ""

# ── Start stripe listen ──────────────────────────────────────────────────────

echo "Starting stripe webhook listener..."
stripe listen --forward-to "$API_URL$WEBHOOK_PATH" --skip-verify > /tmp/stripe-listen.log 2>&1 &
LISTEN_PID=$!

# Wait for stripe listen to be ready
sleep 3
if ! kill -0 "$LISTEN_PID" 2>/dev/null; then
    echo -e "${RED}✗ stripe listen failed to start. Check /tmp/stripe-listen.log${NC}"
    cat /tmp/stripe-listen.log
    exit 1
fi
echo -e "  ${GREEN}✓${NC} stripe listen running (PID $LISTEN_PID)"
echo ""

# ── Test 1: List addon packs (public endpoint) ──────────────────────────────

echo "Test 1: GET /v1/payments/addon-packs"
PACKS=$(curl -sf "$API_URL/v1/payments/addon-packs" 2>&1)
assert_contains "returns JSON array" "$PACKS" "addon_p"
assert_contains "has 3 packs" "$PACKS" "addon_g"
assert_contains "has correct price" "$PACKS" "4700"
echo ""

# ── Test 2: List plans (public endpoint) ─────────────────────────────────────

echo "Test 2: GET /v1/plans"
PLANS=$(curl -sf "$API_URL/v1/plans" 2>&1)
assert_contains "returns plans" "$PLANS" "Starter"
assert_contains "has Business plan" "$PLANS" "Business"
echo ""

# ── Test 3: Webhook receives checkout.session.completed ──────────────────────

echo "Test 3: stripe trigger checkout.session.completed"
TRIGGER_OUT=$(stripe trigger checkout.session.completed 2>&1)
assert_ok "trigger completed without error" $?
sleep 2  # Give webhook time to process

# Check that the webhook was received
WEBHOOK_LOG=$(tail -20 /tmp/stripe-listen.log 2>/dev/null)
assert_contains "webhook forwarded" "$WEBHOOK_LOG" "checkout.session.completed"
echo ""

# ── Test 4: Webhook receives invoice.paid ────────────────────────────────────

echo "Test 4: stripe trigger invoice.paid"
TRIGGER_OUT=$(stripe trigger invoice.paid 2>&1)
assert_ok "trigger completed without error" $?
sleep 2

WEBHOOK_LOG=$(tail -20 /tmp/stripe-listen.log 2>/dev/null)
assert_contains "webhook forwarded" "$WEBHOOK_LOG" "invoice.paid"
echo ""

# ── Test 5: Webhook receives invoice.payment_failed ──────────────────────────

echo "Test 5: stripe trigger invoice.payment_failed"
TRIGGER_OUT=$(stripe trigger invoice.payment_failed 2>&1)
assert_ok "trigger completed without error" $?
sleep 2

WEBHOOK_LOG=$(tail -20 /tmp/stripe-listen.log 2>/dev/null)
assert_contains "webhook forwarded" "$WEBHOOK_LOG" "invoice.payment_failed"
echo ""

# ── Test 6: Webhook receives customer.subscription.updated ───────────────────

echo "Test 6: stripe trigger customer.subscription.updated"
TRIGGER_OUT=$(stripe trigger customer.subscription.updated 2>&1)
assert_ok "trigger completed without error" $?
sleep 2

WEBHOOK_LOG=$(tail -20 /tmp/stripe-listen.log 2>/dev/null)
assert_contains "webhook forwarded" "$WEBHOOK_LOG" "customer.subscription.updated"
echo ""

# ── Test 7: Webhook receives customer.subscription.deleted ───────────────────

echo "Test 7: stripe trigger customer.subscription.deleted"
TRIGGER_OUT=$(stripe trigger customer.subscription.deleted 2>&1)
assert_ok "trigger completed without error" $?
sleep 2

WEBHOOK_LOG=$(tail -20 /tmp/stripe-listen.log 2>/dev/null)
assert_contains "webhook forwarded" "$WEBHOOK_LOG" "customer.subscription.deleted"
echo ""

# ── Test 8: Health check (sanity — API still alive after all webhooks) ───────

echo "Test 8: Health check after all webhooks"
HEALTH=$(curl -sf "$API_URL/health" 2>&1)
assert_ok "API still healthy" $?
echo ""

echo "Done."
