Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepositoryRoot {
    $root = (& git rev-parse --show-toplevel 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($root)) {
        throw "Current directory is not inside a Git repository."
    }
    return [IO.Path]::GetFullPath($root.Trim())
}

function Assert-Command {
    param([Parameter(Mandatory)][string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

function Invoke-Native {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [Parameter()][string[]]$Arguments = @(),
        [Parameter()][string]$Label = ""
    )

    if ($Label) {
        Write-Host ""
        Write-Host "== $Label ==" -ForegroundColor Cyan
    }

    Write-Host "> $FilePath $($Arguments -join ' ')" -ForegroundColor DarkGray
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code ${LASTEXITCODE}: $FilePath $($Arguments -join ' ')"
    }
}

function Ensure-Directory {
    param([Parameter(Mandatory)][string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Write-JsonUtf8NoBom {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)]$Value
    )

    $parent = Split-Path -Parent $Path
    if ($parent) {
        Ensure-Directory -Path $parent
    }

    $json = $Value | ConvertTo-Json -Depth 20
    [IO.File]::WriteAllText(
        $Path,
        $json + [Environment]::NewLine,
        [Text.UTF8Encoding]::new($false)
    )
}

function Test-HttpOk {
    param(
        [Parameter(Mandatory)][string]$Url,
        [int]$TimeoutSec = 5
    )

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec $TimeoutSec
        return ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400)
    }
    catch {
        return $false
    }
}

function Wait-DockerStable {
    param(
        [int]$TimeoutSec = 90,
        [int]$RequiredConsecutiveSuccesses = 3
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $successes = 0

    while ((Get-Date) -lt $deadline) {
        # Docker can emit non-fatal daemon warnings on stderr. With the
        # toolkit's strict error preference, suppress only this probe output.
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            & docker info *> $null
        }
        finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
        if ($LASTEXITCODE -eq 0) {
            $successes++
            if ($successes -ge $RequiredConsecutiveSuccesses) {
                return $true
            }
        }
        else {
            $successes = 0
        }
        Start-Sleep -Seconds 3
    }

    return $false
}

function Get-GitStatusPaths {
    $lines = @(& git status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to read Git status."
    }

    $paths = foreach ($line in $lines) {
        if ($line.Length -lt 4) { continue }
        $path = $line.Substring(3).Trim()
        if ($path -match " -> ") {
            $path = ($path -split " -> ")[-1]
        }
        $path
    }

    return @($paths)
}

function Test-PathAllowed {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter()][string[]]$AllowedPatterns = @()
    )

    $normalizedPath = $Path.Replace("\", "/")
    foreach ($pattern in $AllowedPatterns) {
        $normalizedPattern = $pattern.Replace("\", "/")
        if ($normalizedPath -like $normalizedPattern) {
            return $true
        }
    }
    return $false
}
