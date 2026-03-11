# Bash / Shell Examples

All examples assume `TALLY_URL` and `API_KEY` are set in your environment, or substitute them inline.

```bash
export TALLY_URL="http://localhost:9200"
export API_KEY="your-api-key"
```

---

## Minimal push

The only required fields are `name` and `value`. Type defaults to `gauge`.

```bash
curl -s -X POST "$TALLY_URL/push" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"backup_status","value":1}'
```

---

## With labels

Each unique label set is tracked as a separate series.

```bash
curl -s -X POST "$TALLY_URL/push" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"backup_status","value":1,"labels":{"host":"web-01","env":"prod"}}'
```

---

## Full payload

```bash
curl -s -X POST "$TALLY_URL/push" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name":   "backup_duration_seconds",
    "value":  142.5,
    "type":   "gauge",
    "labels": {"host":"web-01","job":"nightly-backup"}
  }'
```

---

## Reusable function

Drop this into your script or `.bashrc` / `.bash_profile`:

```bash
tally_push() {
  local name="$1"
  local value="$2"
  local labels="${3:-}"      # optional: '{"host":"web-01"}'
  local type="${4:-gauge}"

  local payload
  if [[ -n "$labels" ]]; then
    payload=$(printf '{"name":"%s","value":%s,"type":"%s","labels":%s}' \
      "$name" "$value" "$type" "$labels")
  else
    payload=$(printf '{"name":"%s","value":%s,"type":"%s"}' \
      "$name" "$value" "$type")
  fi

  curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$TALLY_URL/push" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "$payload"
}
```

Usage:

```bash
# Simple
tally_push backup_status 1

# With labels
tally_push backup_status 1 '{"host":"web-01"}'

# Counter with labels
tally_push files_processed_total 4821 '{"host":"web-01"}' counter
```

---

## Example: wrap a job and report outcome

```bash
#!/usr/bin/env bash
set -euo pipefail

TALLY_URL="http://localhost:9200"
API_KEY="your-api-key"

start=$(date +%s%N)

# --- your job here ---
/opt/scripts/run-backup.sh
status=$?
# ----------------------

end=$(date +%s%N)
duration=$(echo "scale=3; ($end - $start) / 1000000000" | bc)

curl -s -X POST "$TALLY_URL/push" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"backup_duration_seconds\",\"value\":$duration,\"labels\":{\"host\":\"$(hostname)\"}}"

curl -s -X POST "$TALLY_URL/push" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"backup_exit_code\",\"value\":$status,\"labels\":{\"host\":\"$(hostname)\"}}"
```

---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.

```bash
KEY="$API_KEY"
URL="$TALLY_URL/push"

curl -s -X POST "$URL" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"name":"req_duration_bucket","value":0,"type":"histogram","labels":{"le":"0.05"}}'

curl -s -X POST "$URL" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"name":"req_duration_bucket","value":3,"type":"histogram","labels":{"le":"0.1"}}'

curl -s -X POST "$URL" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"name":"req_duration_bucket","value":12,"type":"histogram","labels":{"le":"+Inf"}}'

curl -s -X POST "$URL" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"name":"req_duration_sum","value":2.53,"type":"histogram"}'

curl -s -X POST "$URL" -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"name":"req_duration_count","value":12,"type":"histogram"}'
```
