# Bash Examples

## Required Tools
```bash
# curl is required
```

## Retrieving ENV Variables
```bash
TALLY_URL="https://tally.localdomain.com:9200"
TALLY_KEY="$TALLY_KEY"
```

## Sending Without Helper Function
```bash
# Minimal Push
curl -s -X POST "$TALLY_URL/push" \
    -H "Authorization: Bearer $TALLY_KEY" \
    -H "Content-Type: application/json" \
    -d '{"name":"backup_status","value":1}'

# With Labels
curl -s -X POST "$TALLY_URL/push" \
    -H "Authorization: Bearer $TALLY_KEY" \
    -H "Content-Type: application/json" \
    -d '{"name":"backup_status","value":1,"labels":{"host":"web-01","env":"prod"}}'
```

### Helper Function Example
Example Function assumes `TALLY_URL` and `TALLY_KEY` are environment variables
```bash
push_metric() {
    local name="$1"
    local value="$2"
    local metric_type="${3:-gauge}"
    local labels="$4"

    local payload
    if [[ -n "$labels" ]]; then
        payload=$(printf '{"name":"%s","value":%s,"type":"%s","labels":%s}' \
            "$name" "$value" "$metric_type" "$labels")
    else
        payload=$(printf '{"name":"%s","value":%s,"type":"%s"}' \
            "$name" "$value" "$metric_type")
    fi

    curl -s -X POST "$TALLY_URL/push" \
        -H "Authorization: Bearer $TALLY_KEY" \
        -H "Content-Type: application/json" \
        -d "$payload"
}


# Pushing Using Function
push_metric \
    "backup_status" \
    1 \
    "gauge" \
    '{"instance":"immich"}'
```
---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.
> Example is using the helper function above

```bash
names=("req_duration_bucket" "req_duration_bucket" "req_duration_bucket" "req_duration_sum"  "req_duration_count")
values=(0 3 12 2.53 12)
types=("histogram" "histogram" "histogram" "histogram" "histogram")
labels=('{"le":"0.05"}' '{"le":"0.1"}' '{"le":"+Inf"}' '' '')

for i in "${!names[@]}"; do
    push_metric "${names[$i]}" "${values[$i]}" "${types[$i]}" "${labels[$i]}"
done
```


