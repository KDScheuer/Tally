# PowerShell Examples

## Retrieving ENV Variables
```powershell
$TALLY_URL = "https://tally.localdomain.com:9200"
$TALLY_KEY = $env:TALLY_KEY
```

## Sending Without Helper Function
```powershell
# Minimal Push
Invoke-RestMethod "$TALLY_URL/push" -Method POST `
    -Headers @{ Authorization = "Bearer $TALLY_KEY" } `
    -ContentType "application/json" `
    -Body '{"name":"backup_status","value":1}'

# With Labels
Invoke-RestMethod "$TALLY_URL/push" -Method POST `
    -Headers @{ Authorization = "Bearer $TALLY_KEY" } `
    -ContentType "application/json" `
    -Body '{"name":"backup_status","value":1,"labels":{"host":"web-01","env":"prod"}}'
```

### Helper Function Example
Example Function assumes `$TALLY_URL` and `$TALLY_KEY` are global variables
```powershell
function Push-Metric {
    param(
        [string]$name,
        [double]$value,
        [string]$metric_type = "gauge",
        [hashtable]$labels   = @{}
    )
    $payload = @{
        name   = $name
        value  = $value
        type   = $metric_type
        labels = $labels
    }
    Invoke-RestMethod "$TALLY_URL/push" -Method POST `
        -Headers @{ Authorization = "Bearer $TALLY_KEY" } `
        -ContentType "application/json" `
        -Body ($payload | ConvertTo-Json -Compress)
}


# Pushing Using Function
Push-Metric `
    -name        "backup_status" `
    -value       1 `
    -metric_type "gauge" `
    -labels      @{ instance = "immich" }
```
---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.
> Example is using the helper function above

```powershell
$series = @(
    @{ name = "req_duration_bucket"; value = 0;    type = "histogram"; labels = @{ le = "0.05" } }
    @{ name = "req_duration_bucket"; value = 3;    type = "histogram"; labels = @{ le = "0.1"  } }
    @{ name = "req_duration_bucket"; value = 12;   type = "histogram"; labels = @{ le = "+Inf" } }
    @{ name = "req_duration_sum";    value = 2.53; type = "histogram" }
    @{ name = "req_duration_count";  value = 12;   type = "histogram" }
)

foreach ($s in $series) {
    Push-Metric `
        -name        $s.name `
        -value       $s.value `
        -metric_type $s.type `
        -labels      $s.labels
}
```
