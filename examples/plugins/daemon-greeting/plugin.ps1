$Command = $args[0]
$AddrFlag = $args[1]
$Addr = $args[2]

if ($Command -ne "serve" -or $AddrFlag -ne "--addr" -or [string]::IsNullOrWhiteSpace($Addr)) {
  Write-Error "usage: plugin.ps1 serve --addr 127.0.0.1:<port>"
  exit 2
}

$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://$Addr/")
$listener.Start()
Write-Host "daemon greeting plugin listening on $Addr"

while ($listener.IsListening) {
  $ctx = $listener.GetContext()
  $path = $ctx.Request.Url.AbsolutePath
  try {
    if ($path -eq "/health") {
      $body = [Text.Encoding]::UTF8.GetBytes("ok")
      $ctx.Response.StatusCode = 200
      $ctx.Response.OutputStream.Write($body, 0, $body.Length)
      continue
    }

    if ($path -ne "/execute") {
      $ctx.Response.StatusCode = 404
      continue
    }

    $reader = [IO.StreamReader]::new($ctx.Request.InputStream, [Text.Encoding]::UTF8)
    $payload = $reader.ReadToEnd() | ConvertFrom-Json
    $name = [string]$payload.params.name
    if ([string]::IsNullOrWhiteSpace($name)) {
      $name = "Puppet"
    }

    $response = [pscustomobject]@{
      output = @{
        message = "Hello from daemon, $name"
        nodeType = $payload.nodeType
      }
      logs = @(
        @{
          stream = "stdout"
          content = "daemon greeting generated"
        }
      )
    } | ConvertTo-Json -Depth 8 -Compress

    $body = [Text.Encoding]::UTF8.GetBytes($response)
    $ctx.Response.ContentType = "application/json"
    $ctx.Response.StatusCode = 200
    $ctx.Response.OutputStream.Write($body, 0, $body.Length)
  } catch {
    $body = [Text.Encoding]::UTF8.GetBytes($_.Exception.Message)
    $ctx.Response.StatusCode = 500
    $ctx.Response.OutputStream.Write($body, 0, $body.Length)
  } finally {
    $ctx.Response.Close()
  }
}
