param(
  [ValidateSet('inspect','find','stop')][string]$Action,
  [int]$ProcessId,
  [Parameter(Mandatory=$true)][string]$Executable,
  [string]$Birth,
  [string]$CommandHash
)
$ErrorActionPreference = 'Stop'
$module = 'app.workflows.characters.worker'
function Identity($item, $started) {
  if ($null -eq $item -or $item.ExecutablePath -ne $Executable -or
      $item.CommandLine -notmatch '\s-m\s+app\.workflows\.characters\.worker(?:\s|$)') { return $null }
  $bytes = [System.Text.Encoding]::UTF8.GetBytes($item.CommandLine)
  $digest = [Convert]::ToHexString([System.Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
  return @{ pid=[int]$item.ProcessId; birth=$started; commandHash=$digest }
}
try {
  if ($Action -eq 'find') {
    $items = @(Get-CimInstance Win32_Process -Filter "Name = 'python.exe'" | ForEach-Object {
      $candidate = Get-Process -Id $_.ProcessId -ErrorAction SilentlyContinue
      if ($null -ne $candidate) { Identity $_ $candidate.StartTime.ToUniversalTime().Ticks.ToString() }
    } | Where-Object { $null -ne $_ })
    ConvertTo-Json -InputObject $items -Compress
    exit 0
  }
  if ($ProcessId -lt 1) { throw 'invalid process id' }
  $process = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
  if ($null -eq $process) { 'null'; exit 0 }
  # Retain the OS process handle before checking its identity. A reused numeric
  # PID must never authorize stopping a different process or its descendants.
  $handle = $process.Handle
  $item = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId"
  $identity = Identity $item $process.StartTime.ToUniversalTime().Ticks.ToString()
  if ($null -eq $identity) { '{"foreign":true}'; exit 0 }
  if ($Action -eq 'stop') {
    if ($identity.birth -ne $Birth -or $identity.commandHash -ne $CommandHash -or
        $process.StartTime.ToUniversalTime().Ticks.ToString() -ne $Birth) { throw 'process ownership changed' }
    $process.Kill($true)
    if (-not $process.WaitForExit(5000)) { throw 'process did not stop' }
  }
  $identity | ConvertTo-Json -Compress
} catch {
  # No command lines, private configuration, provider errors or credentials.
  [Console]::Error.WriteLine('Character consumer process ownership could not be verified.')
  exit 1
}
