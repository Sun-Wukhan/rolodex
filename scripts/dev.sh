#!/usr/bin/env bash
# Runs the whole stack natively (no Docker): mock vendors, seed, API on SQLite
# and the Vite dev server. Ctrl-C stops everything.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "Missing .env - run 'make env' and edit the change-me values." >&2
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

for port in 8080 9001 9002 5173; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "Port $port is already in use (is 'make up' or 'make k8s-forward' running?)." >&2
    exit 1
  fi
done

pids=()
cleanup() {
  trap - EXIT INT TERM
  echo
  echo "Stopping..."
  kill "${pids[@]}" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Profiles and credentials (password hashes) live in separate database files.
export DB_DRIVER=sqlite DATABASE_URL="${LOCAL_SQLITE_PATH:-rolodex.db}" \
  CREDENTIALS_DATABASE_URL="${LOCAL_SQLITE_CREDENTIALS_PATH:-rolodex-credentials.db}"

echo "==> Building Go binaries"
mkdir -p bin
go build -o bin/ ./cmd/...

echo "==> Starting mock vendors on :9001 (ABC) and :9002 (XYC)"
MOCK_ABC_USERNAME="$ABC_USERNAME" MOCK_ABC_PASSWORD="$ABC_PASSWORD" \
  MOCK_XYC_USERNAME="$XYC_USERNAME" MOCK_XYC_PASSWORD="$XYC_PASSWORD" \
  ./bin/mockvendors 2>&1 | sed 's/^/[vendors] /' &
pids+=("$!")

echo "==> Seeding $DATABASE_URL (profiles) and $CREDENTIALS_DATABASE_URL (credentials)"
./bin/seed 2>&1 | sed 's/^/[seed] /'

echo "==> Starting API on :8080"
ABC_BASE_URL=http://localhost:9001 XYC_BASE_URL=http://localhost:9002 \
  CORS_ALLOWED_ORIGINS=http://localhost:5173 \
  ./bin/api 2>&1 | sed 's/^/[api] /' &
pids+=("$!")

echo "==> Starting web on :5173"
(cd web && { [[ -d node_modules ]] || npm ci; } && VITE_API_URL=http://localhost:8080 npm run dev -- --strictPort) \
  2>&1 | sed 's/^/[web] /' &
pids+=("$!")

echo
echo "Rolodex is starting: open http://localhost:5173"
echo "Sign in as admin (or ada, grace, alan, katherine) with password: $SEED_PASSWORD"
wait
