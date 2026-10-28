#!/usr/bin/env bash
#
# Replider E2E — Pre-flight check for required services.
#
# Usage:
#   ./scripts/start-services.sh
#
# Environment variables:
#   API_URL  — API base URL  (default: http://localhost:8080/v1)
#   APP_URL  — App base URL  (default: http://localhost:3000)

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080/v1}"
API_BASE_URL="${API_URL%/v*}" # strip /v1 to get base URL for /health
APP_URL="${APP_URL:-http://localhost:3000}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "============================================"
echo "  Replider E2E — Service Health Check"
echo "============================================"
echo ""

# -----------------------------------------------------------
# 1. Check API
# -----------------------------------------------------------
echo -n "Checking API at ${API_BASE_URL}/health ... "
if curl -sf "${API_BASE_URL}/health" > /dev/null 2>&1; then
  echo -e "${GREEN}OK${NC}"
else
  echo -e "${RED}FAILED${NC}"
  echo ""
  echo -e "${YELLOW}The API is not reachable at ${API_BASE_URL}/health${NC}"
  echo ""
  echo "To start the API:"
  echo "  cd api/"
  echo "  make docker-dev    # Starts Postgres + API with hot reload"
  echo ""
  echo "Or set API_URL to point to a running instance:"
  echo "  export API_URL=http://your-server:8080/v1"
  echo ""
  exit 1
fi

# -----------------------------------------------------------
# 2. Check Flutter Web App
# -----------------------------------------------------------
echo -n "Checking App at ${APP_URL} ... "
if curl -sf "${APP_URL}" > /dev/null 2>&1; then
  echo -e "${GREEN}OK${NC}"
else
  echo -e "${RED}FAILED${NC}"
  echo ""
  echo -e "${YELLOW}The Flutter web app is not reachable at ${APP_URL}${NC}"
  echo ""
  echo "To start the Flutter web app:"
  echo "  cd app/"
  echo "  flutter run -d chrome --web-port=3000 \\"
  echo "    --dart-define=API_BASE_URL=http://localhost:8080/v1"
  echo ""
  echo "Or just run: npm run test:demo (Playwright auto-serves the built app)"
  echo ""
  echo "To serve manually (if already built):"
  echo "  npx serve app/build/web -l 3000 --no-clipboard"
  echo ""
  echo "Or set APP_URL to point to a running instance:"
  echo "  export APP_URL=http://your-server:3000"
  echo ""
  exit 1
fi

# -----------------------------------------------------------
# 3. All good
# -----------------------------------------------------------
echo ""
echo -e "${GREEN}All services are running!${NC}"
echo ""
echo "You can now run the e2e tests:"
echo "  npm test              # All tests"
echo "  npm run test:demo     # Demo flow (headed)"
echo "  npm run test:video    # Demo flow (video capture)"
echo ""
