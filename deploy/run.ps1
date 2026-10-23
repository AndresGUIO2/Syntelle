[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("bootstrap", "foundation", "services", "all")]
    [string] $Stack
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$saved = @{}

function Set-DeploymentSecret([string] $Name, [string] $Prompt) {
    if (Test-Path "Env:$Name") {
        return
    }
    $secure = Read-Host -Prompt $Prompt -AsSecureString
    $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer)
        if ([string]::IsNullOrWhiteSpace($plain)) {
            throw "$Name is required for stack '$Stack'."
        }
        $script:saved[$Name] = $null
        [Environment]::SetEnvironmentVariable($Name, $plain, "Process")
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
        $plain = $null
    }
}

try {
    if ($Stack -in @("services", "all")) {
        Set-DeploymentSecret "CLOUDFLARE_API_TOKEN" "Cloudflare API token"
    }
    Push-Location $root
    try {
        & python deploy.py apply --stack $Stack --config .deploy-private/config.json
        if ($LASTEXITCODE -ne 0) {
            throw "Deployment script failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    foreach ($name in $saved.Keys) {
        [Environment]::SetEnvironmentVariable($name, $null, "Process")
    }
}
