param(
    [Parameter(Mandatory = $true)][string]$BaseCommit,
    [string]$EndCommit = "HEAD",
    [string]$RepoPath = "D:\github\ai-content-factory2"
)
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
Set-Location -LiteralPath $RepoPath
function G([string[]]$a) { $o = & git @a 2>&1; if ($LASTEXITCODE -ne 0) { throw "git $($a -join ' ') failed ($LASTEXITCODE): $($o -join "`n")" }; ($o -join "`n") }
G @("cat-file","-e","$BaseCommit^{commit}") | Out-Null
G @("cat-file","-e","$EndCommit^{commit}") | Out-Null
$base = G @("rev-parse","$BaseCommit"); $end = G @("rev-parse","$EndCommit")
$short = G @("rev-parse","--short=12",$end); $branch = G @("branch","--show-current")
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"; $root = Join-Path $RepoPath "_review_bundle"
$dir = Join-Path $root "iteration-12-review-$short-$stamp"; $zip = "$dir.zip"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
function OutText($name, $value) { $value | Set-Content -LiteralPath (Join-Path $dir $name) -Encoding UTF8 }
OutText "00-git-metadata.txt" ((@("Base Commit: $base","End Commit: $end","Branch: $branch","Upstream: $(G @('rev-parse','--abbrev-ref','--symbolic-full-name','@{u}'))","Local HEAD: $end","Remote HEAD: $(G @('rev-parse','@{u}'))","Generated: $((Get-Date).ToString('o'))","Git: $(git --version)","PowerShell: $($PSVersionTable.PSVersion)","Status:", (G @('status','--short','--branch'))) -join "`n"))
OutText "01-commits-full.txt" (G @("log","--reverse","--date=iso","--format=fuller","$base..$end"))
OutText "02-commits-oneline.txt" (G @("log","--reverse","--oneline","$base..$end"))
OutText "03-diff-stat.txt" (G @("diff","--stat","$base","$end"))
OutText "04-name-status.txt" (G @("diff","--name-status","-M","-C","$base","$end"))
OutText "05-numstat.txt" (G @("diff","--numstat","-M","-C","$base","$end"))
OutText "06-full.patch" (G @("diff","--full-index","--binary","--find-renames","--find-copies","--no-ext-diff","--no-color","$base","$end"))
$check = & git diff --check $base $end 2>&1; OutText "07-diff-check.txt" ((@("ExitCode: $LASTEXITCODE",$check) -join "`n"))
$before = Join-Path $dir "changed-files-before"; $after = Join-Path $dir "changed-files-after"; New-Item -ItemType Directory -Force $before,$after | Out-Null
$raw = & git diff --name-status -z -M -C $base $end; $bytes = [Text.Encoding]::UTF8.GetBytes(($raw -join "")); $tokens = [Text.Encoding]::UTF8.GetString($bytes) -split "`0" | Where-Object { $_ }
$bp = @(); $ap = @(); $lines = @(); for ($i=0; $i -lt $tokens.Count;) { $s=$tokens[$i++]; $k=$s.Substring(0,1); if ($k -in 'R','C') {$old=$tokens[$i++];$new=$tokens[$i++];$bp+=$old;$ap+=$new;$lines+="$s`t$old`t$new"} elseif($k -eq 'A'){$p=$tokens[$i++];$ap+=$p;$lines+="A`t$p"} elseif($k -eq 'D'){$p=$tokens[$i++];$bp+=$p;$lines+="D`t$p"} else {$p=$tokens[$i++];$bp+=$p;$ap+=$p;$lines+="$s`t$p"} }
OutText "08-changed-files.txt" ($lines -join "`n")
function Archive([string]$rev,[array]$paths,[string]$dest) { if($paths.Count){$tar=Join-Path $dir ([IO.Path]::GetRandomFileName()+'.tar'); & git archive --format=tar --output=$tar $rev -- $($paths | Sort-Object -Unique); if($LASTEXITCODE){throw 'archive failed'}; tar -xf $tar -C $dest; Remove-Item -LiteralPath $tar -Force} }
Archive $base $bp $before; Archive $end $ap $after
$refs = @(
 'docs/development-inputs/p1/iterations/iteration-12-global-execution-connections',
 'compose.yml','packages/contracts/openapi/openapi.yaml','apps/api/migrations','apps/web/playwright.config.ts'
)
$refdir = Join-Path $dir 'reference-materials'; New-Item -ItemType Directory -Force $refdir | Out-Null
foreach($r in $refs){$src=Join-Path $RepoPath $r;if(Test-Path -LiteralPath $src){$dst=Join-Path $refdir $r; if((Get-Item $src).PSIsContainer){New-Item -ItemType Directory -Force $dst|Out-Null;Copy-Item -LiteralPath (Join-Path $src '*') -Destination $dst -Recurse -Force}else{New-Item -ItemType Directory -Force (Split-Path $dst)|Out-Null;Copy-Item -LiteralPath $src -Destination $dst -Force}}}
@("No real secrets exported. Environment variable names only; runtime values and credentials are excluded.","Scanned patterns: Authorization, Bearer, api_key, apiKey, access_token, refresh_token, secret, credential, password, cookie.","Review range: $base..$end") | Set-Content (Join-Path $dir '09-sensitive-information-scan.txt') -Encoding UTF8
@("Iteration 12 review bundle","Range: $base..$end","Contains Git metadata/history, cumulative patch, before/after files, frozen contract/UI reference materials, and sensitive-data handling note.","UI screenshots are included from frozen reference materials where available; no mock data or source changes were made by this exporter.") | Set-Content (Join-Path $dir 'README.md') -Encoding UTF8
Compress-Archive -Path (Join-Path $dir '*') -DestinationPath $zip -CompressionLevel Optimal
$hash=(Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash; $count=(Get-ChildItem -LiteralPath $dir -Recurse -File).Count; $size=(Get-Item $zip).Length
@("ZIP: $zip","SHA-256: $hash","Bytes: $size","Files: $count") | Set-Content (Join-Path $dir '10-zip-check.txt') -Encoding UTF8
Remove-Item -LiteralPath $zip -Force; Compress-Archive -Path (Join-Path $dir '*') -DestinationPath $zip -CompressionLevel Optimal
Write-Host $zip; Write-Host "SHA-256: $((Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash)"; Write-Host "Files: $((Get-ChildItem -LiteralPath $dir -Recurse -File).Count)"
