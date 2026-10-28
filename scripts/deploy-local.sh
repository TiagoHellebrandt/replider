#!/usr/bin/env bash
# =============================================================================
# deploy-local.sh — build and ship the API from the VPS itself.
#
# The recipe the team used by hand whenever GitHub Actions could not run
# (billing block, runner outage, SSH i/o timeouts), written down once so it
# is the same every time. READ docs/deploy-emergencia.md FIRST: this is the
# emergency path, it needs the local gates green, an issue and a tag.
#
# What it does, in order (every step idempotent; re-running is safe):
#   1. exports the exact SHA's api/ tree (git archive — never the working
#      tree, so an unstaged edit can never ship);
#   2. generates api/docs (swag v1; gitignored, imported by main.go);
#   3. docker build tagged ghcr.io/caritium/replider-api:sha-<7>;
#   4. pins API_IMAGE_TAG (and RELEASE_SHA) in the stack's .env;
#   5. docker compose up -d --no-deps api;
#   6. waits for GET /health = 200 (timeout, then dumps the logs);
#   7. nginx -s reload (otherwise the proxy keeps the old upstream);
#   8. prunes dangling images (the VPS disk is small);
#   9. smoke: GET /v1/plans through the public URL.
#
# Usage (on the VPS, from a checkout of the repo):
#   scripts/deploy-local.sh <sha>            # deploy that commit (must be on main)
#   STACK_DIR=~/replider scripts/deploy-local.sh <sha>
#
# Environment:
#   STACK_DIR     where docker-compose.prod.yml + .env live (default ~/replider)
#   PUBLIC_URL    smoke-test origin (default https://api.replider.com)
#   HEALTH_TIMEOUT seconds to wait for /health (default 180)
#   SKIP_MAIN_CHECK=1 allow a SHA that is not on origin/main (DON'T, unless
#                 the incident is exactly "main is broken and this is the fix")
# =============================================================================
set -euo pipefail

SHA_ARG="${1:-}"
STACK_DIR="${STACK_DIR:-$HOME/replider}"
PUBLIC_URL="${PUBLIC_URL:-https://api.replider.com}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-180}"
IMAGE="ghcr.io/caritium/replider-api"

log()  { printf '[deploy-local] %s\n' "$*"; }
fail() { printf '[deploy-local] ERROR: %s\n' "$*" >&2; exit 1; }

[ -n "${SHA_ARG}" ] || fail "usage: $0 <git-sha>"
command -v docker >/dev/null || fail "docker not found"
command -v git    >/dev/null || fail "git not found"
command -v curl   >/dev/null || fail "curl not found"

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || fail "run from inside the repo checkout"
cd "${REPO_ROOT}"

[ -f "${STACK_DIR}/docker-compose.prod.yml" ] || fail "no docker-compose.prod.yml in ${STACK_DIR} (set STACK_DIR)"
[ -f "${STACK_DIR}/.env" ] || fail "no .env in ${STACK_DIR} — the stack was never deployed here"

# ── 0. Resolve and validate the SHA ──────────────────────────────────────────
git fetch --quiet origin main || log "warning: could not fetch origin/main (offline?) — using the local refs"
SHA="$(git rev-parse --verify "${SHA_ARG}^{commit}")" || fail "unknown commit ${SHA_ARG}"
SHORT="${SHA:0:7}"
TAG="sha-${SHORT}"
if [ "${SKIP_MAIN_CHECK:-0}" != "1" ]; then
  git merge-base --is-ancestor "${SHA}" origin/main \
    || fail "${SHORT} is not on origin/main. Emergency deploys ship main only (docs/deploy-emergencia.md). SKIP_MAIN_CHECK=1 overrides — write down why in the issue."
fi
log "deploying ${SHA} as ${IMAGE}:${TAG}"

# ── 1. Export the exact tree ─────────────────────────────────────────────────
BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/replider-apibuild.XXXXXX")"
trap 'rm -rf "${BUILD_DIR}"' EXIT
git archive "${SHA}" api | tar -x -C "${BUILD_DIR}"
log "exported api/ to ${BUILD_DIR}"

# ── 2. Generate api/docs (gitignored, imported by main.go) ───────────────────
# Same target the CI image build runs; swag is installed on demand.
if ! command -v swag >/dev/null; then
  log "installing swag (v1) for the embedded Swagger UI"
  GOBIN="${BUILD_DIR}/bin" go install github.com/swaggo/swag/cmd/swag@v1.16.4
  export PATH="${BUILD_DIR}/bin:${PATH}"
fi
( cd "${BUILD_DIR}/api" && make swagger >/dev/null ) || fail "make swagger failed — the image cannot be built without api/docs"

# ── 3. Build the image ───────────────────────────────────────────────────────
if docker image inspect "${IMAGE}:${TAG}" >/dev/null 2>&1; then
  log "image ${IMAGE}:${TAG} already exists locally — reusing (delete it to force a rebuild)"
else
  docker build -t "${IMAGE}:${TAG}" "${BUILD_DIR}/api" || fail "docker build failed"
fi

# ── 4. Pin the tag in .env ───────────────────────────────────────────────────
ENV_FILE="${STACK_DIR}/.env"
pin_var() {
  local key="$1" value="$2"
  if grep -qE "^${key}=" "${ENV_FILE}"; then
    sed -i.bak -E "s|^${key}=.*|${key}=\"${value}\"|" "${ENV_FILE}" && rm -f "${ENV_FILE}.bak"
  else
    printf '%s="%s"\n' "${key}" "${value}" >> "${ENV_FILE}"
  fi
}
pin_var API_IMAGE_TAG "${TAG}"
pin_var RELEASE_SHA "${TAG}"
chmod 600 "${ENV_FILE}"
log "pinned API_IMAGE_TAG=${TAG} in ${ENV_FILE}"

# ── 5. Roll the api container ────────────────────────────────────────────────
cd "${STACK_DIR}"
docker compose --env-file .env -f docker-compose.prod.yml up -d --no-deps api \
  || fail "docker compose up failed"

# ── 6. Wait for /health ──────────────────────────────────────────────────────
log "waiting up to ${HEALTH_TIMEOUT}s for the API to answer /health ..."
deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
healthy=""
while [ "$(date +%s)" -lt "${deadline}" ]; do
  state="$(docker inspect -f '{{.State.Status}}' replider_api 2>/dev/null || echo missing)"
  if [ "${state}" = "exited" ] || [ "${state}" = "dead" ]; then
    break
  fi
  code="$(docker exec replider_api wget -qO- --server-response http://localhost:8080/health 2>&1 | awk '/HTTP\//{print $2}' | tail -1 || true)"
  if [ "${code}" = "200" ]; then healthy=yes; break; fi
  sleep 3
done
if [ -z "${healthy}" ]; then
  echo "---------- docker logs replider_api (tail 200) ----------" >&2
  docker logs --tail 200 replider_api >&2 || true
  fail "API did not become healthy within ${HEALTH_TIMEOUT}s. The previous image is still tagged locally; re-run with the previous SHA to roll back."
fi
log "API healthy on ${TAG}"

# ── 7. Reload nginx ──────────────────────────────────────────────────────────
if docker exec replider_nginx nginx -t >/dev/null 2>&1; then
  docker exec replider_nginx nginx -s reload && log "nginx reloaded"
else
  log "nginx config test failed; recreating the proxy container"
  docker compose --env-file .env -f docker-compose.prod.yml up -d --no-deps --force-recreate nginx
fi

# ── 8. Prune dangling images ─────────────────────────────────────────────────
docker image prune -f >/dev/null && log "dangling images pruned"

# ── 9. Smoke ─────────────────────────────────────────────────────────────────
smoke="$(curl -fsS -o /dev/null -w '%{http_code}' "${PUBLIC_URL}/v1/plans" || true)"
[ "${smoke}" = "200" ] || fail "smoke GET ${PUBLIC_URL}/v1/plans returned '${smoke}' (expected 200). The container is up; check nginx/DNS."
log "smoke OK: GET ${PUBLIC_URL}/v1/plans → 200"

cat <<EOF
[deploy-local] DONE — ${IMAGE}:${TAG} is live.
[deploy-local] Now, per docs/deploy-emergencia.md:
[deploy-local]   git tag deploy-emergencia-$(date +%Y%m%d)-${TAG} ${SHA} && git push origin --tags
[deploy-local]   and close the loop: re-run the pipeline on this commit once Actions is back.
EOF
