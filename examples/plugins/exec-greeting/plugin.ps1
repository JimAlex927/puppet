param([string]$Command)

if ($Command -ne "execute") {
  Write-Error "unsupported command: $Command"
  exit 2
}

$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$name = [string]$payload.params.name
if ([string]::IsNullOrWhiteSpace($name)) {
  $name = "Puppet"
}

[pscustomobject]@{
  output = @{
    message = "Hello, $name"
    workspace = $payload.workspace
  }
  logs = @(
    @{
      stream = "stdout"
      content = "exec greeting generated"
    }
  )
} | ConvertTo-Json -Depth 8 -Compress
