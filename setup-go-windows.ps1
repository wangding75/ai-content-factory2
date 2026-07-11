#requires -Version 5.1
<#
.SYNOPSIS
  Checks and prepares a Go development environment on Windows.

.DESCRIPTION
  - Detects the Go version required by go.work / go.mod / apps/api/go.mod.
  - Checks the installed Go toolchain.
  - Installs Go through WinGet when Go is missing.
  - Upgrades Go through WinGet when the installed version is lower than the project requirement.
  - Ensures %USERPROFILE%\go\bin is present in the user PATH.
  - Optionally sets GOPROXY.
  - Optionally downloads modules and runs Go tests.
  - Writes a JSON environment report.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\setup-go-windows.ps1

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\setup-go-windows.ps1 -ProjectRoot D:\workspace\ai-content-factory -VerifyProject

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\setup-go-windows.ps1 -CheckOnly

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\setup-go-windows.ps1 -GoProxy "https://proxy.golang.org,direct"
#>

[CmdletBinding()]
param(
    [Parameter()]
    [string]$ProjectRoot = (Get-Location).Path,

    [Parameter()]
    [switch]$CheckOnly,

    [Parameter()]
    [switch]$VerifyProject,

    [Parameter()]
    [string]$GoProxy = "",

    [Parameter()]
    [string]$ReportPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host "[PASS] $Message" -ForegroundColor Green
}

function Write-WarnMessage {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Fail {
    param([string]$Message)
    Write-Host "[FAIL] $Message" -ForegroundColor Red
}

function Refresh-ProcessPath {
    $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")

    $parts = @()
    if (-not [string]::IsNullOrWhiteSpace($machinePath)) {
        $parts += $machinePath
    }
    if (-not [string]::IsNullOrWhiteSpace($userPath)) {
        $parts += $userPath
    }

    $env:Path = $parts -join ";"
}

function Add-UserPathEntry {
    param([Parameter(Mandatory = $true)][string]$PathEntry)

    $normalizedEntry = $PathEntry.TrimEnd("\")
    $currentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")

    $entries = @()
    if (-not [string]::IsNullOrWhiteSpace($currentUserPath)) {
        $entries = $currentUserPath.Split(";") |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    }

    $exists = $entries | Where-Object {
        $_.TrimEnd("\").Equals($normalizedEntry, [StringComparison]::OrdinalIgnoreCase)
    }

    if (-not $exists) {
        $newEntries = @($entries) + $normalizedEntry
        [Environment]::SetEnvironmentVariable(
            "Path",
            ($newEntries -join ";"),
            "User"
        )
        Write-Ok "Added '$normalizedEntry' to the user PATH."
    }
    else {
        Write-Ok "'$normalizedEntry' is already present in the user PATH."
    }

    Refresh-ProcessPath
}

function Convert-ToComparableVersion {
    param([Parameter(Mandatory = $true)][string]$Text)

    $match = [regex]::Match($Text, '(?:go)?(?<version>\d+\.\d+(?:\.\d+)?)')
    if (-not $match.Success) {
        throw "Unable to parse Go version from '$Text'."
    }

    $versionText = $match.Groups["version"].Value
    $segments = $versionText.Split(".")
    if ($segments.Count -eq 2) {
        $versionText = "$versionText.0"
    }

    return [version]$versionText
}

function Get-GoRequirement {
    param([Parameter(Mandatory = $true)][string]$Root)

    $candidateFiles = @(
        (Join-Path $Root "go.work"),
        (Join-Path $Root "go.mod"),
        (Join-Path $Root "apps\api\go.mod")
    ) | Select-Object -Unique

    $requirements = @()

    foreach ($file in $candidateFiles) {
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
            continue
        }

        $content = Get-Content -LiteralPath $file

        foreach ($line in $content) {
            if ($line -match '^\s*toolchain\s+go(?<version>\d+\.\d+(?:\.\d+)?)\s*$') {
                $requirements += [pscustomobject]@{
                    Source  = $file
                    Kind    = "toolchain"
                    Version = Convert-ToComparableVersion $Matches["version"]
                }
            }
            elseif ($line -match '^\s*go\s+(?<version>\d+\.\d+(?:\.\d+)?)\s*$') {
                $requirements += [pscustomobject]@{
                    Source  = $file
                    Kind    = "go"
                    Version = Convert-ToComparableVersion $Matches["version"]
                }
            }
        }
    }

    if ($requirements.Count -eq 0) {
        return $null
    }

    return $requirements |
        Sort-Object -Property Version -Descending |
        Select-Object -First 1
}

function Get-GoCommand {
    Refresh-ProcessPath

    $command = Get-Command "go.exe" -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $commonPaths = @(
        (Join-Path $env:ProgramFiles "Go\bin\go.exe")
    )

    if (${env:ProgramFiles(x86)}) {
        $commonPaths += (Join-Path ${env:ProgramFiles(x86)} "Go\bin\go.exe")
    }

    foreach ($path in $commonPaths) {
        if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) {
            $goBin = Split-Path -Parent $path
            $env:Path = "$goBin;$env:Path"
            return $path
        }
    }

    return $null
}

function Invoke-WinGet {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    $winget = Get-Command "winget.exe" -ErrorAction SilentlyContinue
    if (-not $winget) {
        throw @"
WinGet is not available.
Install or update Microsoft App Installer, reopen PowerShell, and rerun this script.
"@
    }

    & $winget.Source @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "WinGet failed with exit code $LASTEXITCODE."
    }
}

function Install-GoWithWinGet {
    Write-Step "Installing the latest stable Go toolchain with WinGet"

    Invoke-WinGet @(
        "install",
        "--id", "GoLang.Go",
        "--exact",
        "--source", "winget",
        "--accept-package-agreements",
        "--accept-source-agreements",
        "--silent"
    )

    Refresh-ProcessPath
}

function Upgrade-GoWithWinGet {
    Write-Step "Upgrading the Go toolchain with WinGet"

    Invoke-WinGet @(
        "upgrade",
        "--id", "GoLang.Go",
        "--exact",
        "--source", "winget",
        "--accept-package-agreements",
        "--accept-source-agreements",
        "--silent"
    )

    Refresh-ProcessPath
}

function Invoke-Go {
    param(
        [Parameter(Mandatory = $true)][string]$GoExe,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    & $GoExe @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Go command failed: go $($Arguments -join ' ') (exit code $LASTEXITCODE)."
    }
}

if ($env:OS -ne "Windows_NT") {
    throw "This script only supports Windows."
}

$resolvedProjectRoot = [IO.Path]::GetFullPath($ProjectRoot)
if (-not (Test-Path -LiteralPath $resolvedProjectRoot -PathType Container)) {
    throw "ProjectRoot does not exist: $resolvedProjectRoot"
}

if ([string]::IsNullOrWhiteSpace($ReportPath)) {
    $ReportPath = Join-Path $resolvedProjectRoot ".ai-dev\reports\go-environment.json"
}
$resolvedReportPath = [IO.Path]::GetFullPath($ReportPath)

$report = [ordered]@{
    generated_at_utc        = [DateTime]::UtcNow.ToString("o")
    computer_name           = $env:COMPUTERNAME
    windows_version         = [Environment]::OSVersion.VersionString
    process_architecture    = if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITECTURE)) {
        $env:PROCESSOR_ARCHITECTURE
    }
    elseif ([Environment]::Is64BitProcess) {
        "x64"
    }
    else {
        "x86"
    }
    project_root            = $resolvedProjectRoot
    check_only              = [bool]$CheckOnly
    project_verification    = [bool]$VerifyProject
    required_go_version     = $null
    required_version_source = $null
    installed_go_version    = $null
    go_executable           = $null
    go_env                  = [ordered]@{}
    module_download         = "not_requested"
    go_test                 = "not_requested"
    result                  = "failed"
}

try {
    Write-Step "Inspecting project Go version requirements"

    $requirement = Get-GoRequirement -Root $resolvedProjectRoot
    if ($requirement) {
        $requiredVersion = [version]$requirement.Version
        $report.required_go_version = $requiredVersion.ToString()
        $report.required_version_source = $requirement.Source
        Write-Ok "Required Go version: $requiredVersion ($($requirement.Kind), $($requirement.Source))"
    }
    else {
        $requiredVersion = $null
        Write-WarnMessage "No Go version directive found in go.work, go.mod, or apps/api/go.mod."
    }

    Write-Step "Checking the installed Go toolchain"

    $goExe = Get-GoCommand

    if (-not $goExe) {
        if ($CheckOnly) {
            throw "Go is not installed or is not available on PATH."
        }

        Install-GoWithWinGet
        $goExe = Get-GoCommand

        if (-not $goExe) {
            throw "Go installation completed, but go.exe is still unavailable. Open a new PowerShell window and rerun the script."
        }
    }

    $versionOutput = & $goExe version
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to execute '$goExe version'."
    }

    $installedVersion = Convert-ToComparableVersion $versionOutput
    $report.installed_go_version = $installedVersion.ToString()
    $report.go_executable = $goExe

    Write-Ok "$versionOutput"
    Write-Ok "Go executable: $goExe"

    if ($requiredVersion -and $installedVersion -lt $requiredVersion) {
        $message = "Installed Go $installedVersion is lower than required Go $requiredVersion."

        if ($CheckOnly) {
            throw $message
        }

        Write-WarnMessage $message
        Upgrade-GoWithWinGet

        $goExe = Get-GoCommand
        $versionOutput = & $goExe version
        if ($LASTEXITCODE -ne 0) {
            throw "Unable to execute Go after upgrade."
        }

        $installedVersion = Convert-ToComparableVersion $versionOutput
        $report.installed_go_version = $installedVersion.ToString()
        $report.go_executable = $goExe

        if ($installedVersion -lt $requiredVersion) {
            throw "Go upgrade did not satisfy the requirement. Installed=$installedVersion, Required=$requiredVersion."
        }

        Write-Ok "Go upgraded successfully: $versionOutput"
    }
    elseif ($requiredVersion) {
        Write-Ok "Installed Go satisfies the project requirement."
    }

    Write-Step "Checking GOPATH and PATH"

    $goPath = (& $goExe env GOPATH).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goPath)) {
        throw "Unable to resolve GOPATH."
    }

    if (-not (Test-Path -LiteralPath $goPath)) {
        if ($CheckOnly) {
            Write-WarnMessage "GOPATH directory does not exist: $goPath"
        }
        else {
            New-Item -ItemType Directory -Path $goPath -Force | Out-Null
            Write-Ok "Created GOPATH directory: $goPath"
        }
    }

    $goBinPath = Join-Path $goPath "bin"
    if (-not (Test-Path -LiteralPath $goBinPath) -and -not $CheckOnly) {
        New-Item -ItemType Directory -Path $goBinPath -Force | Out-Null
    }

    if ($CheckOnly) {
        $pathEntries = $env:Path.Split(";") |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) }

        $goBinOnPath = $pathEntries | Where-Object {
            $_.TrimEnd("\").Equals($goBinPath.TrimEnd("\"), [StringComparison]::OrdinalIgnoreCase)
        }

        if ($goBinOnPath) {
            Write-Ok "$goBinPath is present on PATH."
        }
        else {
            Write-WarnMessage "$goBinPath is not present on PATH."
        }
    }
    else {
        Add-UserPathEntry -PathEntry $goBinPath
    }

    if (-not [string]::IsNullOrWhiteSpace($GoProxy)) {
        if ($CheckOnly) {
            Write-WarnMessage "CheckOnly is enabled; GOPROXY will not be changed."
        }
        else {
            Write-Step "Configuring GOPROXY"
            Invoke-Go -GoExe $goExe -Arguments @("env", "-w", "GOPROXY=$GoProxy")
            Write-Ok "GOPROXY configured as '$GoProxy'."
        }
    }

    Write-Step "Collecting Go environment information"

    $envNames = @(
        "GOOS",
        "GOARCH",
        "GOROOT",
        "GOPATH",
        "GOBIN",
        "GOMODCACHE",
        "GOPROXY",
        "GOSUMDB",
        "CGO_ENABLED"
    )

    foreach ($name in $envNames) {
        $value = (& $goExe env $name).Trim()
        if ($LASTEXITCODE -ne 0) {
            throw "Unable to read 'go env $name'."
        }
        $report.go_env[$name] = $value
        Write-Host ("{0,-14} {1}" -f $name, $value)
    }

    if ($VerifyProject) {
        $goWorkFile = Join-Path $resolvedProjectRoot "go.work"
        $rootGoMod = Join-Path $resolvedProjectRoot "go.mod"
        $apiGoMod = Join-Path $resolvedProjectRoot "apps\api\go.mod"

        if (Test-Path -LiteralPath $apiGoMod -PathType Leaf) {
            $moduleRoot = Split-Path -Parent $apiGoMod
        }
        elseif (Test-Path -LiteralPath $rootGoMod -PathType Leaf) {
            $moduleRoot = $resolvedProjectRoot
        }
        else {
            $moduleRoot = $null
        }

        if (-not $moduleRoot) {
            Write-WarnMessage "Project verification skipped because neither apps/api/go.mod nor a root go.mod was found."
            $report.module_download = "skipped_no_module"
            $report.go_test = "skipped_no_module"
        }
        else {
            if (Test-Path -LiteralPath $goWorkFile -PathType Leaf) {
                Write-Step "Synchronizing the Go workspace"
                Push-Location $resolvedProjectRoot
                try {
                    Invoke-Go -GoExe $goExe -Arguments @("work", "sync")
                    Write-Ok "Go workspace synchronization passed."
                }
                finally {
                    Pop-Location
                }
            }

            Write-Step "Downloading Go modules"
            Push-Location $moduleRoot
            try {
                Invoke-Go -GoExe $goExe -Arguments @("mod", "download")
                $report.module_download = "passed"
                Write-Ok "Go module download passed."

                Write-Step "Running Go tests"
                Invoke-Go -GoExe $goExe -Arguments @("test", "./...")
                $report.go_test = "passed"
                Write-Ok "Go tests passed."
            }
            finally {
                Pop-Location
            }
        }
    }

    $report.result = "passed"
    Write-Step "Go environment check completed"
    Write-Ok "Result: PASS"
}
catch {
    $report.result = "failed"
    $report.error = $_.Exception.Message
    Write-Fail $_.Exception.Message
    throw
}
finally {
    $reportDirectory = Split-Path -Parent $resolvedReportPath
    if (-not (Test-Path -LiteralPath $reportDirectory)) {
        New-Item -ItemType Directory -Path $reportDirectory -Force | Out-Null
    }

    $report |
        ConvertTo-Json -Depth 8 |
        Set-Content -LiteralPath $resolvedReportPath -Encoding UTF8

    Write-Host ""
    Write-Host "Environment report: $resolvedReportPath" -ForegroundColor DarkGray
}
