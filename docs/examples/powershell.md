# PowerShell Examples

All examples assume `$TallyUrl` and `$ApiKey` are set, or substitute them inline.

```powershell
$TallyUrl = "http://localhost:9200"
$ApiKey   = "your-api-key"
```

---

## Minimal push

The only required fields are `name` and `value`. Type defaults to `gauge`.

```powershell
Invoke-RestMethod "$TallyUrl/push" -Method POST `
  -Headers @{ Authorization = "Bearer $ApiKey" } `
  -ContentType "application/json" `
  -Body '{"name":"backup_status","value":1}'
```

---

## With labels

Each unique label set is tracked as a separate series.

```powershell
Invoke-RestMethod "$TallyUrl/push" -Method POST `
  -Headers @{ Authorization = "Bearer $ApiKey" } `
  -ContentType "application/json" `
  -Body '{"name":"backup_status","value":1,"labels":{"host":"win-01","env":"prod"}}'
```

---

## Full payload

```powershell
$body = @{
    name   = "backup_duration_seconds"
    value  = 142.5
    type   = "gauge"
    labels = @{ host = "win-01"; job = "nightly-backup" }
} | ConvertTo-Json

Invoke-RestMethod "$TallyUrl/push" -Method POST `
  -Headers @{ Authorization = "Bearer $ApiKey" } `
  -ContentType "application/json" `
  -Body $body
```

---

## Reusable function

Drop this into your `$PROFILE` or source it at the top of a script:

```powershell
function Push-TallyMetric {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][double]$Value,
        [hashtable]$Labels  = @{},
        [string]$Type       = "gauge",
        [string]$Url        = $env:TALLY_URL,
        [string]$ApiKey     = $env:API_KEY
    )

    $payload = @{ name = $Name; value = $Value; type = $Type }
    if ($Labels.Count -gt 0) { $payload.labels = $Labels }

    try {
        Invoke-RestMethod "$Url/push" -Method POST `
            -Headers @{ Authorization = "Bearer $ApiKey" } `
            -ContentType "application/json" `
            -Body ($payload | ConvertTo-Json -Compress)
    } catch {
        Write-Warning "Tally push failed: $_"
    }
}
```

Usage:

```powershell
# Simple
Push-TallyMetric -Name backup_status -Value 1

# With labels
Push-TallyMetric -Name backup_status -Value 1 -Labels @{ host = "win-01" }

# Counter with labels
Push-TallyMetric -Name files_processed_total -Value 4821 `
    -Labels @{ host = "win-01" } -Type counter
```

---

## Reading `API_KEY` from environment

Avoid hardcoding your key — store it as a user or system environment variable:

```powershell
# Set once (persists across sessions)
[System.Environment]::SetEnvironmentVariable("API_KEY", "your-api-key", "User")

# Use in scripts
Push-TallyMetric -Name backup_status -Value 1 -ApiKey $env:API_KEY
```

---

## Example: wrap a job and report outcome

```powershell
$TallyUrl = "http://localhost:9200"
$ApiKey   = $env:API_KEY
$Hostname = $env:COMPUTERNAME

$start = Get-Date

try {
    # --- your job here ---
    & C:\Scripts\Run-Backup.ps1
    $exitCode = 0
} catch {
    $exitCode = 1
    Write-Warning "Job failed: $_"
}

$duration = ((Get-Date) - $start).TotalSeconds

Push-TallyMetric -Name backup_duration_seconds -Value $duration `
    -Labels @{ host = $Hostname } -Url $TallyUrl -ApiKey $ApiKey

Push-TallyMetric -Name backup_exit_code -Value $exitCode `
    -Labels @{ host = $Hostname } -Url $TallyUrl -ApiKey $ApiKey
```

---

## Histogram series

Each histogram component is a separate push. Tally groups them by family name automatically.

```powershell
$headers = @{ Authorization = "Bearer $ApiKey" }
$url     = "$TallyUrl/push"

@(
    @{ name="req_duration_bucket"; value=0;    type="histogram"; labels=@{le="0.05"} }
    @{ name="req_duration_bucket"; value=3;    type="histogram"; labels=@{le="0.1"}  }
    @{ name="req_duration_bucket"; value=12;   type="histogram"; labels=@{le="+Inf"} }
    @{ name="req_duration_sum";    value=2.53; type="histogram" }
    @{ name="req_duration_count";  value=12;   type="histogram" }
) | ForEach-Object {
    Invoke-RestMethod $url -Method POST -Headers $headers `
        -ContentType "application/json" -Body ($_ | ConvertTo-Json -Compress)
}
```
