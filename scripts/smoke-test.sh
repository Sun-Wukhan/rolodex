#!/usr/bin/env bash
# Post-deployment smoke test: exercises the deployed API end to end (health,
# auth, search, create, get, enrich) plus the web front end's security headers.
#
# Usage: SMOKE_PASSWORD=... scripts/smoke-test.sh
#   API_URL         API base URL                  (default http://localhost:8080)
#   WEB_URL         web front end; skipped if empty (default empty)
#   SMOKE_USERNAME  seeded account to log in as    (default admin)
#   SMOKE_PASSWORD  that account's password        (required)
# Writes a Markdown report to $GITHUB_STEP_SUMMARY when set.
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
WEB_URL="${WEB_URL:-}"
SMOKE_USERNAME="${SMOKE_USERNAME:-admin}"
: "${SMOKE_PASSWORD:?SMOKE_PASSWORD is required}"

for bin in curl jq openssl; do
  command -v "$bin" >/dev/null || { echo "missing dependency: $bin" >&2; exit 2; }
done

passed=0
failed=0
report=()
body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT

# record NAME OK [DETAIL] - tallies one check and prints a result line.
record() {
  local name="$1" ok="$2" detail="${3:-}"
  if [[ "$ok" == "true" ]]; then
    passed=$((passed + 1))
    report+=("| ✅ | ${name} | ${detail} |")
    printf '  PASS  %s %s\n' "$name" "$detail"
  else
    failed=$((failed + 1))
    report+=("| ❌ | ${name} | ${detail} |")
    printf '  FAIL  %s %s\n' "$name" "$detail" >&2
  fi
}

# call METHOD URL [CURL_ARGS...] - performs a request, leaves the body in
# $body_file and prints the HTTP status code.
call() {
  local method="$1" url="$2"
  shift 2
  curl -sS -o "$body_file" -w '%{http_code}' --max-time 20 -X "$method" "$url" "$@" || echo 000
}

# expect NAME WANT GOT - records whether an HTTP status matched.
expect() {
  local name="$1" want="$2" got="$3"
  if [[ "$got" == "$want" ]]; then record "$name" true "HTTP $got"; else record "$name" false "want HTTP $want, got $got: $(head -c 200 "$body_file")"; fi
}

# wait_ready URL - polls until the URL returns 200 or roughly 60s elapse.
wait_ready() {
  local url="$1"
  for _ in $(seq 1 30); do
    [[ "$(call GET "$url")" == "200" ]] && return 0
    sleep 2
  done
  return 1
}

echo "==> Smoke testing ${API_URL}"
if wait_ready "${API_URL}/readyz"; then record "API becomes ready" true; else record "API becomes ready" false "timed out"; fi

expect "GET /healthz" 200 "$(call GET "${API_URL}/healthz")"
expect "GET /readyz" 200 "$(call GET "${API_URL}/readyz")"

status="$(call GET "${API_URL}/api/v1/me")"
expect "GET /me without a token is rejected" 401 "$status"

login_body="$(jq -n --arg u "$SMOKE_USERNAME" --arg p "$SMOKE_PASSWORD" '{username: $u, password: $p}')"
bad_login_body="$(jq -n --arg u "$SMOKE_USERNAME" --arg p "$(openssl rand -hex 16)" '{username: $u, password: $p}')"
expect "POST /auth/login with a wrong password" 401 \
  "$(call POST "${API_URL}/api/v1/auth/login" -H 'Content-Type: application/json' --data "$bad_login_body")"

status="$(call POST "${API_URL}/api/v1/auth/login" -H 'Content-Type: application/json' --data "$login_body")"
expect "POST /auth/login" 200 "$status"
token="$(jq -r '.access_token // empty' "$body_file")"
if [[ -z "$token" ]]; then
  record "access token issued" false "no access_token in response"
  token="invalid"
else
  record "access token issued" true
fi
auth=(-H "Authorization: Bearer ${token}")

status="$(call GET "${API_URL}/api/v1/me" "${auth[@]}")"
expect "GET /me" 200 "$status"
record "token identifies the caller" "$(jq --arg u "$SMOKE_USERNAME" '.username == $u' "$body_file")"

expect "GET /users without filters is rejected" 400 "$(call GET "${API_URL}/api/v1/users" "${auth[@]}")"

status="$(call GET "${API_URL}/api/v1/users?name=katherine" "${auth[@]}")"
expect "GET /users?name=katherine" 200 "$status"
katherine_id="$(jq -r '.data[0].user_id // empty' "$body_file")"
record "search finds the seeded profile" "$([[ -n "$katherine_id" ]] && echo true || echo false)"

username="smoke-$(openssl rand -hex 4)"
create_body="$(jq -n --arg u "$username" --arg p "$(openssl rand -hex 16)" \
  '{name: "Smoke Test", phone: "416-555-0199", username: $u, password: $p}')"
status="$(call POST "${API_URL}/api/v1/users" "${auth[@]}" -H 'Content-Type: application/json' --data "$create_body")"
expect "POST /users" 201 "$status"
new_id="$(jq -r '.id // empty' "$body_file")"

if [[ -n "$new_id" ]]; then
  status="$(call GET "${API_URL}/api/v1/users/${new_id}" "${auth[@]}")"
  expect "GET /users/{id}" 200 "$status"
  record "phone normalised to E.164" "$(jq '.profile.phone == "+14165550199"' "$body_file")"
  record "credential secrets are never returned" "$(jq '[.. | objects | has("password") or has("secret") or has("hash")] | any | not' "$body_file")"
  expect "duplicate username is rejected" 409 \
    "$(call POST "${API_URL}/api/v1/users" "${auth[@]}" -H 'Content-Type: application/json' --data "$create_body")"
fi

expect "GET /users/{unknown} is 404" 404 \
  "$(call GET "${API_URL}/api/v1/users/00000000-0000-4000-8000-000000000000" "${auth[@]}")"

if [[ -n "$katherine_id" ]]; then
  status="$(call POST "${API_URL}/api/v1/users/${katherine_id}/enrich" "${auth[@]}")"
  expect "POST /users/{id}/enrich" 200 "$status"
  # XYC deliberately fails a share of calls, so only ABC must succeed.
  record "ABC provider enriched the profile" "$(jq '[.providers[] | select(.provider == "abc" and .status == "ok")] | length == 1' "$body_file")"
  record "every provider reported a known status" \
    "$(jq '[.providers[].status] | all(. == "ok" or . == "not_found" or . == "unavailable")' "$body_file")"
fi

if [[ -n "$WEB_URL" ]]; then
  echo "==> Smoke testing ${WEB_URL}"
  headers="$(curl -sS -D - -o /dev/null --max-time 20 "${WEB_URL}/" || true)"
  record "web serves index.html" "$(grep -qE '^HTTP/[0-9.]+ 200' <<<"$headers" && echo true || echo false)"
  for header in Content-Security-Policy X-Frame-Options X-Content-Type-Options Referrer-Policy; do
    record "web sends ${header}" "$(grep -qi "^${header}:" <<<"$headers" && echo true || echo false)"
  done
  deep_link="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "${WEB_URL}/users/does-not-exist" || echo 000)"
  record "SPA deep links fall back to index.html" "$([[ "$deep_link" == "200" ]] && echo true || echo false)" "HTTP ${deep_link}"
fi

echo
echo "Smoke test: ${passed} passed, ${failed} failed"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### Smoke test: ${API_URL}"
    echo
    echo "**${passed} passed, ${failed} failed**"
    echo
    echo "| | Check | Detail |"
    echo "|---|---|---|"
    printf '%s\n' "${report[@]}"
  } >>"$GITHUB_STEP_SUMMARY"
fi

[[ "$failed" -eq 0 ]]
