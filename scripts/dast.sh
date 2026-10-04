#!/usr/bin/env bash
# Dynamic application security testing (DAST) with OWASP ZAP against a running
# deployment:
#   1. API scan  - authenticated active scan driven by api/openapi.yaml
#   2. Web scan  - passive baseline scan of the web front end
# Any alert not triaged in .zap/*-rules.tsv fails the run, so new findings
# block the pipeline until they are fixed or explicitly accepted.
#
# Usage: DAST_PASSWORD=... scripts/dast.sh
#   API_URL         API base URL                          (default http://localhost:8080)
#   WEB_URL         web front end; skipped if empty        (default http://localhost:3000)
#   DAST_USERNAME   account used to obtain a bearer token  (default admin)
#   DAST_PASSWORD   that account's password                (required)
#   DAST_REPORT_DIR where HTML/JSON/Markdown reports go    (default dast-reports)
#   ZAP_IMAGE       ZAP container image                    (default zaproxy/zap-stable:2.17.0)
set -euo pipefail

cd "$(dirname "$0")/.."

API_URL="${API_URL:-http://localhost:8080}"
WEB_URL="${WEB_URL-http://localhost:3000}"
DAST_USERNAME="${DAST_USERNAME:-admin}"
: "${DAST_PASSWORD:?DAST_PASSWORD is required}"
DAST_REPORT_DIR="${DAST_REPORT_DIR:-dast-reports}"
ZAP_IMAGE="${ZAP_IMAGE:-zaproxy/zap-stable:2.17.0}"

for bin in curl jq docker; do
  command -v "$bin" >/dev/null || { echo "missing dependency: $bin" >&2; exit 2; }
done

# Containers reach the host's port-forwards via the host network on Linux and
# via host.docker.internal on Docker Desktop.
docker_net=()
container_url() { printf '%s' "$1"; }
if [[ "$(uname -s)" == "Darwin" ]]; then
  container_url() { printf '%s' "${1/localhost/host.docker.internal}"; }
else
  docker_net=(--network host)
fi

mkdir -p "$DAST_REPORT_DIR"
report_abs="$(cd "$DAST_REPORT_DIR" && pwd)"
cp api/openapi.yaml .zap/api-rules.tsv .zap/web-rules.tsv "$DAST_REPORT_DIR/"
# The ZAP image runs as an unprivileged user that must write reports here.
chmod a+rwx "$DAST_REPORT_DIR"

echo "==> Obtaining a bearer token from ${API_URL}"
login_body="$(jq -n --arg u "$DAST_USERNAME" --arg p "$DAST_PASSWORD" '{username: $u, password: $p}')"
token="$(curl -sS --fail --max-time 20 -X POST "${API_URL}/api/v1/auth/login" \
  -H 'Content-Type: application/json' --data "$login_body" | jq -r '.access_token // empty')"
[[ -n "$token" ]] || { echo "login failed; cannot run an authenticated scan" >&2; exit 2; }

# zap NAME SCRIPT ARGS... - runs one packaged ZAP scan and echoes its exit
# code: 0 clean, 1 FAIL alerts, 2 WARN alerts, 3 scanner error.
zap() {
  local name="$1"
  shift
  local code=0
  docker run --rm ${docker_net[@]+"${docker_net[@]}"} \
    -v "${report_abs}:/zap/wrk:rw" \
    -e ZAP_AUTH_HEADER=Authorization \
    -e ZAP_AUTH_HEADER_VALUE="Bearer ${token}" \
    -e ZAP_AUTH_HEADER_SITE="$(container_url "$API_URL" | sed -E 's#^https?://##; s#/.*$##')" \
    "$ZAP_IMAGE" "$@" >"$DAST_REPORT_DIR/${name}.log" 2>&1 || code=$?
  grep -E '^(FAIL|WARN)-NEW|^FAIL-NEW: [0-9]' "$DAST_REPORT_DIR/${name}.log" >&2 || true
  echo "$code"
}

echo "==> ZAP API scan (authenticated, OpenAPI) against ${API_URL}"
api_code="$(zap api zap-api-scan.py -t openapi.yaml -f openapi -O "$(container_url "$API_URL")" \
  -c api-rules.tsv -J api.json -r api.html -w api.md)"
echo "    exit code ${api_code}"

web_code=0
if [[ -n "$WEB_URL" ]]; then
  echo "==> ZAP baseline scan against ${WEB_URL}"
  web_code="$(zap web zap-baseline.py -t "$(container_url "$WEB_URL")" \
    -c web-rules.tsv -J web.json -r web.html -w web.md)"
  echo "    exit code ${web_code}"
fi

# describe CODE - turns a ZAP exit code into a short verdict.
describe() {
  case "$1" in
    0) echo "✅ clean" ;;
    1) echo "❌ failing alerts" ;;
    2) echo "❌ untriaged warnings" ;;
    *) echo "⚠️ scanner error (exit $1)" ;;
  esac
}

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### DAST (OWASP ZAP)"
    echo
    echo "| Scan | Target | Result |"
    echo "|---|---|---|"
    echo "| API (authenticated, OpenAPI) | ${API_URL} | $(describe "$api_code") |"
    [[ -n "$WEB_URL" ]] && echo "| Web baseline | ${WEB_URL} | $(describe "$web_code") |"
    echo
    echo "Full HTML/JSON reports are attached to this run as the \`dast-reports\` artifact."
  } >>"$GITHUB_STEP_SUMMARY"
fi

for report in "$DAST_REPORT_DIR"/api.json "$DAST_REPORT_DIR"/web.json; do
  [[ -f "$report" ]] || continue
  jq -r --arg f "$(basename "$report")" \
    '.site[]?.alerts[]? | "\($f): [\(.riskdesc)] \(.name) (\(.pluginid)) x\(.count)"' "$report"
done

[[ "$api_code" == 0 && "$web_code" == 0 ]]
