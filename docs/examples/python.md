# Python Examples

## Required Import
```python
import requests
import os
```

## Retrieving ENV Variables
```python
TALLY_URL   = "https://tally.localdomain.com:9200"
TALLY_KEY   = os.getenv("TALLY_KEY")
```

## Sending Without Helper Function
```python
# Minimal Push
requests.post(
    f"{TALLY_URL}/push",
    headers={"Authorization": f"Bearer {TALLY_KEY}"},
    json={"name": "backup_status", "value": 1}
)

# With Labels
requests.post(
    f"{TALLY_URL}/push",
    headers={"Authorization": f"Bearer {TALLY_KEY}"},
    json={
        "name":   "backup_status",
        "value":  1,
        "labels": {"host": "web-01", "env": "prod"}
    }
)
```

### Helper Function Example
Example Function assumes `TALLY_URL` and `TALLY_KEY` are global variables
```python
def push_metric(name: str, value: float, metric_type: str = "gauge", labels: dict = None) -> requests.Response:
    payload = {
        "name":   name,
        "value":  value,
        "type":   metric_type,
        "labels": labels if labels is not None else {},
    }
    resp = requests.post(
        f"{TALLY_URL}/push",
        headers={"Authorization": f"Bearer {TALLY_KEY}"},
        json=payload,
    )
    resp.raise_for_status()
    return resp


# Pushing Using Function
push_metric(
    name="backup_status",
    value=1,
    metric_type="gauge",
    labels={"instance": "immich"},
)
```
---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.
> Example is using the helper function above

```python
series = [
    {"name": "req_duration_bucket", "value": 0,    "type": "histogram", "labels": {"le": "0.05"}},
    {"name": "req_duration_bucket", "value": 3,    "type": "histogram", "labels": {"le": "0.1"}},
    {"name": "req_duration_bucket", "value": 12,   "type": "histogram", "labels": {"le": "+Inf"}},
    {"name": "req_duration_sum",    "value": 2.53, "type": "histogram"},
    {"name": "req_duration_count",  "value": 12,   "type": "histogram"},
]

for s in series:
    push_metric(
        name         = s.get("name"), 
        value        = s.get("value"), 
        metric_type  = s.get("type"),
        labels=s.get("labels")
    )
```
