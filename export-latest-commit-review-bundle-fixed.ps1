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
    $hasNativePreference = Test-Path variable:PSNativeCommandUseErrorActionPreference

    if ($hasNativePreference) {
        $oldNativePreference = $PSNativeCommandUseErrorActionPreference
        $PSNativeCommandUseErrorActionPreference = $false
    }

    try {
        $output = & git @Arguments 2>&1
        $exitCode = $LASTEXITCODE
        $text = $output -join [Environment]::NewLine
    }
    finally {
        if ($hasNativePreference) {
            $PSNativeCommandUseErrorActionPreference = $oldNativePreference
        }
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

$changedFilesRaw = (
    Invoke-Git -Arguments @(
        "diff",
        "--name-only",
        "--diff-filter=ACMRT",
        "HEAD^",
        "HEAD"
    )
).Output

$changedFiles = @(
    $changedFilesRaw -split "\r?\n" |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_)
        } |
        Sort-Object -Unique
)

$changedFiles |
    Set-Content `
        -LiteralPath (Join-Path $bundleDir "10-changed-files.txt") `
        -Encoding UTF8

if ($changedFiles.Count -gt 0) {
    $afterArchive = Join-Path $bundleDir "after.tar"

    $afterArgs = @(
        "archive",
        "--format=tar",
        "--output=$afterArchive",
        "HEAD",
        "--"
    ) + $changedFiles

    Invoke-Git -Arguments $afterArgs | Out-Null
    tar -xf $afterArchive -C $afterDir
    Remove-Item -LiteralPath $afterArchive -Force

    # 只导出父提交中真实存在的文件。
    # 新增文件不会出现在 HEAD^，因此不能直接用 cat-file -e 检查。
    $beforeExisting = @()

    foreach ($path in $changedFiles) {
        $treeResult = Invoke-Git -Arguments @(
            "ls-tree",
            "-r",
            "--name-only",
            "HEAD^",
            "--",
            $path
        ) -AllowFailure

        $matchedPaths = @(
            $treeResult.Output -split "\r?\n" |
                Where-Object {
                    -not [string]::IsNullOrWhiteSpace($_)
                }
        )

        if ($treeResult.ExitCode -eq 0 -and $matchedPaths -contains $path) {
            $beforeExisting += $path
        }
    }

    if ($beforeExisting.Count -gt 0) {
        $beforeArchive = Join-Path $bundleDir "before.tar"

        $beforeArgs = @(
            "archive",
            "--format=tar",
            "--output=$beforeArchive",
            "HEAD^",
            "--"
        ) + $beforeExisting

        Invoke-Git -Arguments $beforeArgs | Out-Null
        tar -xf $beforeArchive -C $beforeDir
        Remove-Item -LiteralPath $beforeArchive -Force
    }
}

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
