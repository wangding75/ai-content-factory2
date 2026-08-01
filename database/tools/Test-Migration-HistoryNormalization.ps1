param()

$ErrorActionPreference = 'Stop'
$repoRoot = (& git rev-parse --show-toplevel).Trim()
if ($LASTEXITCODE -ne 0) { throw 'not a Git repository' }

$source = Join-Path $repoRoot 'apps\api\migrations'
$validator = Join-Path $repoRoot 'database\tools\Validate-Migration-History.ps1'
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("acf-migration-eol-" + [guid]::NewGuid().ToString('N'))

try {
    foreach ($style in @('lf', 'crlf')) {
        $target = Join-Path $tempRoot $style
        [System.IO.Directory]::CreateDirectory($target) | Out-Null
        [System.IO.File]::Copy(
            (Join-Path $source 'migration-checksums.sha256'),
            (Join-Path $target 'migration-checksums.sha256')
        )

        foreach ($file in Get-ChildItem -LiteralPath $source -File -Filter '*.sql') {
            $text = [System.IO.File]::ReadAllText($file.FullName, [System.Text.Encoding]::UTF8)
            $logical = $text.Replace("`r`n", "`n").Replace("`r", "`n")
            $rendered = if ($style -eq 'crlf') { $logical.Replace("`n", "`r`n") } else { $logical }
            [System.IO.File]::WriteAllText(
                (Join-Path $target $file.Name),
                $rendered,
                (New-Object System.Text.UTF8Encoding($false))
            )
        }

        & $validator -MigrationDirectory $target -ChecksumManifest (Join-Path $target 'migration-checksums.sha256') | Out-Host
        if ($LASTEXITCODE -ne 0) { throw "migration history validation failed for $style" }
        Write-Host "[PASS] All migration files validate from the $style worktree copy."
    }
} finally {
    if ([System.IO.Directory]::Exists($tempRoot)) {
        [System.IO.Directory]::Delete($tempRoot, $true)
    }
}
