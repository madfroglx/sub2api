$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$backendDir = Join-Path $repoRoot "backend"
$bundledGo = Join-Path $repoRoot ".dev\tools\go\bin\go.exe"
$go = if (Test-Path $bundledGo) { $bundledGo } else { "go" }

Push-Location $backendDir
try {
  & $go run ./cmd/reconcile-provider-usage @args
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
