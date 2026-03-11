# Python Examples

All examples use the `requests` library (`pip install requests`). Set your URL and key before running.

```python
TALLY_URL = "http://localhost:9200"
API_KEY   = "your-api-key"
```

---

## Minimal push

The only required fields are `name` and `value`. Type defaults to `gauge`.

```python
import requests

requests.post(
    f"{TALLY_URL}/push",
    headers={"Authorization": f"Bearer {API_KEY}"},
    json={"name": "backup_status", "value": 1}
)
```

---

## With labels

Each unique label set is tracked as a separate series.

```python
requests.post(
    f"{TALLY_URL}/push",
    headers={"Authorization": f"Bearer {API_KEY}"},
    json={
        "name":   "backup_status",
        "value":  1,
        "labels": {"host": "web-01", "env": "prod"}
    }
)
```

---

## Full payload

```python
requests.post(
    f"{TALLY_URL}/push",
    headers={"Authorization": f"Bearer {API_KEY}"},
    json={
        "name":   "backup_duration_seconds",
        "value":  142.5,
        "type":   "gauge",
        "labels": {"host": "web-01", "job": "nightly-backup"}
    }
)
```

---

## Reusable client class

```python
import os
import requests
from typing import Optional

class TallyClient:
    def __init__(
        self,
        url: str = os.getenv("TALLY_URL", "http://localhost:9200"),
        api_key: str = os.getenv("API_KEY", ""),
    ):
        self._url = url.rstrip("/")
        self._headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }

    def push(
        self,
        name: str,
        value: float,
        type: str = "gauge",
        labels: Optional[dict] = None,
    ) -> requests.Response:
        payload: dict = {"name": name, "value": value, "type": type}
        if labels:
            payload["labels"] = labels
        resp = self._session().post(f"{self._url}/push", headers=self._headers, json=payload)
        resp.raise_for_status()
        return resp

    def _session(self) -> requests.Session:
        if not hasattr(self, "_s"):
            self._s = requests.Session()
        return self._s
```

Usage:

```python
tally = TallyClient()

# Simple
tally.push("backup_status", 1)

# With labels
tally.push("backup_status", 1, labels={"host": "web-01"})

# Counter with labels
tally.push("files_processed_total", 4821, type="counter", labels={"host": "web-01"})
```

---

## Reading `API_KEY` from environment

```python
import os

API_KEY   = os.environ["API_KEY"]   # raises if not set — fail loudly
TALLY_URL = os.getenv("TALLY_URL", "http://localhost:9200")
```

---

## Example: wrap a job and report outcome

```python
import os
import time
import requests

TALLY_URL = os.getenv("TALLY_URL", "http://localhost:9200")
API_KEY   = os.environ["API_KEY"]
HOST      = os.uname().nodename  # or platform.node() on Windows

def push(name, value, **labels):
    requests.post(
        f"{TALLY_URL}/push",
        headers={"Authorization": f"Bearer {API_KEY}"},
        json={"name": name, "value": value, "labels": labels},
    )

start = time.time()
exit_code = 0

try:
    run_backup()  # your job here
except Exception as e:
    print(f"Job failed: {e}")
    exit_code = 1

duration = time.time() - start

push("backup_duration_seconds", duration, host=HOST)
push("backup_exit_code", exit_code, host=HOST)
```

---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.

```python
headers = {"Authorization": f"Bearer {API_KEY}"}
url     = f"{TALLY_URL}/push"

series = [
    {"name": "req_duration_bucket", "value": 0,    "type": "histogram", "labels": {"le": "0.05"}},
    {"name": "req_duration_bucket", "value": 3,    "type": "histogram", "labels": {"le": "0.1"}},
    {"name": "req_duration_bucket", "value": 12,   "type": "histogram", "labels": {"le": "+Inf"}},
    {"name": "req_duration_sum",    "value": 2.53, "type": "histogram"},
    {"name": "req_duration_count",  "value": 12,   "type": "histogram"},
]

for s in series:
    requests.post(url, headers=headers, json=s)
```
