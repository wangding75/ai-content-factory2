param(
    [string]$RepoPath = "D:\github\ai-content-factory2"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Invoke-Git {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Arguments,
        [switch]$AllowFailure
    )

    # PowerShell 7 会把原生命令 stderr 转成 ErrorRecord。
    # 临时关闭该行为，统一通过 $LASTEXITCODE 判断成功或失败。
    $oldNativePreference = $null
    $oldErrorActionPreference = $ErrorActionPreference
    $hasNativePreference = Test-Path variable:PSNativeCommandUseErrorActionPreference

    if ($hasNativePreference) {
        $oldNativePreference = $PSNativeCommandUseErrorActionPreference
        $PSNativeCommandUseErrorActionPreference = $false
    }

    try {
        $ErrorActionPreference = "Continue"
        $output = & git @Arguments 2>&1
        $exitCode = $LASTEXITCODE
        $text = $output -join [Environment]::NewLine
    }
    finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
        $ErrorActionPreference = $oldErrorActionPreference
    }

    if (-not $AllowFailure -and $exitCode -ne 0) {
        throw "git $($Arguments -join ' ') 执行失败，退出码：$exitCode`n$text"
    }

    [PSCustomObject]@{
        ExitCode = $exitCode
        Output   = $text
    }
}

if (-not (Test-Path -LiteralPath $RepoPath)) {
    throw "仓库目录不存在：$RepoPath"
}

Set-Location -LiteralPath $RepoPath

$isRepo = Invoke-Git -Arguments @("rev-parse", "--is-inside-work-tree")
if ($isRepo.Output.Trim() -ne "true") {
    throw "指定目录不是 Git 仓库：$RepoPath"
}

# 固定审核最近一次提交：HEAD^..HEAD
$head = (Invoke-Git -Arguments @("rev-parse", "HEAD")).Output.Trim()
$parent = (Invoke-Git -Arguments @("rev-parse", "HEAD^")).Output.Trim()
$shortHead = (Invoke-Git -Arguments @("rev-parse", "--short=12", "HEAD")).Output.Trim()
$branch = (Invoke-Git -Arguments @("branch", "--show-current")).Output.Trim()

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$outputRoot = Join-Path $RepoPath "_review_bundle"
$bundleName = "latest-commit-review-$shortHead-$timestamp"
$bundleDir = Join-Path $outputRoot $bundleName
$zipPath = "$bundleDir.zip"

$beforeDir = Join-Path $bundleDir "changed-files-before"
$afterDir = Join-Path $bundleDir "changed-files-after"

New-Item -ItemType Directory -Path $beforeDir -Force | Out-Null
New-Item -ItemType Directory -Path $afterDir -Force | Out-Null

$upstreamResult = Invoke-Git -Arguments @(
    "rev-parse",
    "--abbrev-ref",
    "--symbolic-full-name",
    "@{u}"
) -AllowFailure

$upstream = if ($upstreamResult.ExitCode -eq 0) {
    $upstreamResult.Output.Trim()
} else {
    ""
}

$upstreamCommitResult = Invoke-Git -Arguments @("rev-parse", "@{u}") -AllowFailure
$upstreamCommit = if ($upstreamCommitResult.ExitCode -eq 0) {
    $upstreamCommitResult.Output.Trim()
} else {
    ""
}

$workingTree = (
    Invoke-Git -Arguments @(
        "status",
        "--porcelain=v1",
        "--untracked-files=all"
    )
).Output.Trim()

$metadata = [ordered]@{
    generatedAt      = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss zzz")
    repository       = $RepoPath
    branch           = $branch
    commit           = $head
    parentCommit     = $parent
    reviewRange      = "$parent..$head"
    upstream         = $upstream
    upstreamCommit   = $upstreamCommit
    pushed           = (
        -not [string]::IsNullOrWhiteSpace($upstreamCommit) -and
        $head -eq $upstreamCommit
    )
    workingTreeClean = [string]::IsNullOrWhiteSpace($workingTree)
}

$metadata |
    ConvertTo-Json -Depth 5 |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "00-metadata.json") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "status",
        "--short",
        "--branch",
        "--untracked-files=all"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "01-git-status.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "show",
        "-s",
        "--decorate=full",
        "--format=fuller",
        "HEAD"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "02-commit-info.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "diff",
        "--name-status",
        "-M",
        "-C",
        "HEAD^",
        "HEAD"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "03-name-status.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "diff",
        "--stat",
        "HEAD^",
        "HEAD"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "04-diff-stat.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "diff",
        "--full-index",
        "--binary",
        "--find-renames",
        "--find-copies",
        "--no-ext-diff",
        "--no-color",
        "HEAD^",
        "HEAD"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "05-full.patch") `
        -Encoding UTF8

$diffCheck = Invoke-Git -Arguments @(
    "diff",
    "--check",
    "HEAD^",
    "HEAD"
) -AllowFailure

@(
    "ExitCode: $($diffCheck.ExitCode)"
    ""
    $diffCheck.Output
) |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "06-diff-check.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @(
        "log",
        "--oneline",
        "--decorate",
        "--graph",
        "-n",
        "10"
    )
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "07-recent-log.txt") `
        -Encoding UTF8

(
    Invoke-Git -Arguments @("branch", "-vv")
).Output |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "08-branch-vv.txt") `
        -Encoding UTF8

@(
    "Branch: $branch"
    "HEAD: $head"
    "Parent: $parent"
    "Upstream: $upstream"
    "UpstreamCommit: $upstreamCommit"
    "Pushed: $($metadata.pushed)"
    "WorkingTreeClean: $($metadata.workingTreeClean)"
) |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "09-push-and-clean-status.txt") `
        -Encoding UTF8

$nameStatusPath = Join-Path $bundleDir "name-status.z"
$nameStatusProcess = [Diagnostics.Process]::new()
$nameStatusProcess.StartInfo.FileName = "git"
$nameStatusProcess.StartInfo.Arguments = "diff --name-status -z -M -C HEAD^ HEAD"
$nameStatusProcess.StartInfo.WorkingDirectory = $RepoPath
$nameStatusProcess.StartInfo.UseShellExecute = $false
$nameStatusProcess.StartInfo.RedirectStandardOutput = $true
$nameStatusProcess.StartInfo.RedirectStandardError = $true
[void]$nameStatusProcess.Start()
$nameStatusStream = [IO.File]::Open($nameStatusPath, [IO.FileMode]::Create, [IO.FileAccess]::Write)
try {
    $nameStatusProcess.StandardOutput.BaseStream.CopyTo($nameStatusStream)
}
finally {
    $nameStatusStream.Dispose()
}
$nameStatusError = $nameStatusProcess.StandardError.ReadToEnd()
$nameStatusProcess.WaitForExit()
if ($nameStatusProcess.ExitCode -ne 0) { throw "git diff --name-status -z 执行失败，退出码：$($nameStatusProcess.ExitCode)`n$nameStatusError" }
$tokens = @([Text.RegularExpressions.Regex]::Split([Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($nameStatusPath)), "`0") | Where-Object { $_.Length -gt 0 })
Remove-Item -LiteralPath $nameStatusPath -Force
$beforeFiles = [System.Collections.Generic.List[string]]::new()
$afterFiles = [System.Collections.Generic.List[string]]::new()
$changedLines = [System.Collections.Generic.List[string]]::new()
for ($index = 0; $index -lt $tokens.Count;) {
    $status = $tokens[$index++]
    $kind = $status.Substring(0, 1)
    switch ($kind) {
        "A" { $afterFiles.Add($tokens[$index]); $changedLines.Add("A`t$($tokens[$index++])") }
        "M" { $beforeFiles.Add($tokens[$index]); $afterFiles.Add($tokens[$index]); $changedLines.Add("M`t$($tokens[$index++])") }
        "D" { $beforeFiles.Add($tokens[$index]); $changedLines.Add("D`t$($tokens[$index++])") }
        "T" { $beforeFiles.Add($tokens[$index]); $afterFiles.Add($tokens[$index]); $changedLines.Add("T`t$($tokens[$index++])") }
        { $_ -in "R", "C" } {
            $oldPath = $tokens[$index++]; $newPath = $tokens[$index++]
            $beforeFiles.Add($oldPath); $afterFiles.Add($newPath)
            $changedLines.Add("$status`t$oldPath`t$newPath")
        }
        default { throw "不支持的 Git 变更状态：$status" }
    }
}
$changedLines | Set-Content -LiteralPath (Join-Path $bundleDir "10-changed-files.txt") -Encoding UTF8

function Export-ChangedFiles([string]$Revision, [System.Collections.Generic.List[string]]$Paths, [string]$Destination, [string]$ArchiveName) {
    $uniquePaths = @($Paths | Sort-Object -Unique)
    if ($uniquePaths.Count -eq 0) { return }
    $archive = Join-Path $bundleDir $ArchiveName
    Invoke-Git -Arguments (@("archive", "--format=tar", "--output=$archive", $Revision, "--") + $uniquePaths) | Out-Null
    tar -xf $archive -C $Destination
    Remove-Item -LiteralPath $archive -Force
}

Export-ChangedFiles "HEAD^" $beforeFiles $beforeDir "before.tar"
Export-ChangedFiles "HEAD" $afterFiles $afterDir "after.tar"

@"
最近一次提交审核包

固定审核范围：
HEAD^..HEAD

本审核包包含：

- 最近一次提交信息；
- Git 状态、分支和远程同步状态；
- 完整变更文件清单；
- 完整 Patch；
- 变更前文件；
- 变更后文件；
- diff --check 结果；
- 最近 10 条提交记录。

脚本不绑定任何 Iteration，也不执行测试。
测试结果应在提交前的执行报告约束审核阶段确认。

请上传整个 ZIP 文件。
"@ |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "README-上传说明.txt") `
        -Encoding UTF8

if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}

Compress-Archive `
    -LiteralPath $bundleDir `
    -DestinationPath $zipPath `
    -CompressionLevel Optimal

Write-Host ""
Write-Host "最近一次提交审核包已生成："
Write-Host $zipPath
Write-Host ""

if (-not $metadata.pushed) {
    Write-Warning "HEAD 与上游分支不一致，请确认 Push 是否成功。"
}

if (-not $metadata.workingTreeClean) {
    Write-Warning "当前工作区不干净，请确认是否存在未提交或未跟踪文件。"
}
