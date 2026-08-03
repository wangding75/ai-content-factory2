[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:18080/api/v1',
    [string]$ProjectId = '10000000-0000-4000-8000-000000000001',
    [string]$ContentItemId = '8a4c8607-d911-4cbb-91e7-f1498b6ff54d',
    [string]$ReviewIssueId = '7addfd92-5df9-4afa-832d-7787d860e505',
    [string]$ReviewReportId = '82ea425f-a932-41d5-9919-2125764ebfdb'
)

$ErrorActionPreference = 'Stop'

function Invoke-Acf {
    param(
        [Parameter(Mandatory = $true)][string]$Method,
        [Parameter(Mandatory = $true)][string]$Path,
        $Body,
        [string]$IdempotencyKey = ''
    )

    $headers = @{}
    if ($IdempotencyKey -ne '') {
        $headers['Idempotency-Key'] = $IdempotencyKey
    }

    $arguments = @{
        Uri = $ApiBaseUrl + $Path
        Method = $Method
        Headers = $headers
        UseBasicParsing = $true
        TimeoutSec = 30
    }
    if ($null -ne $Body) {
        $arguments['ContentType'] = 'application/json'
        $arguments['Body'] = ($Body | ConvertTo-Json -Depth 30 -Compress)
    }
    $response = Invoke-RestMethod @arguments
    return $response.data
}

function Wait-Run {
    param(
        [Parameter(Mandatory = $true)][string]$RunId,
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $run = Invoke-Acf -Method GET -Path "/workflow-runs/$RunId"
        if ($run.status -in @('succeeded', 'failed', 'cancelled', 'timed_out')) {
            return $run
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)

    throw "Run $RunId did not reach a terminal state within $TimeoutSeconds seconds."
}

function Get-Events {
    param([Parameter(Mandatory = $true)][string]$RunId)
    return (Invoke-Acf -Method GET -Path "/workflow-runs/$RunId/events").items
}

function Get-LatestRun {
    param(
        [Parameter(Mandatory = $true)][string]$Stage,
        [Parameter(Mandatory = $true)][string]$SubjectId,
        [Parameter(Mandatory = $true)][string]$SubjectType
    )

    $path = "/workflow-runs?projectId=$ProjectId&stage=$Stage&limit=20&offset=0"
    $items = (Invoke-Acf -Method GET -Path $path).items
    return @($items | Where-Object { $_.subjectId -eq $SubjectId -and $_.subjectType -eq $SubjectType })[0]
}

function Invoke-ReviewNeedsChanges {
    $preflight = Invoke-Acf -Method POST -Path "/content-versions/75b55fe0-9504-4c4d-baec-045aa4c2c168/review-runs/preflight" -Body @{
        sourceContentVersionVersion = 1
        optionalInstructions = '[force-fail] scripted review'
    }
    $run = Invoke-Acf -Method POST -Path "/content-versions/75b55fe0-9504-4c4d-baec-045aa4c2c168/review-runs" -Body @{ preflightToken = $preflight.preflightToken } -IdempotencyKey 'acf-i19-script-review-needs-changes'
    return [pscustomobject]@{
        Preflight = $preflight
        Run = (Wait-Run -RunId $run.id)
        Events = (Get-Events -RunId $run.id)
        Summary = (Invoke-Acf -Method GET -Path "/content-items/$ContentItemId/review-summary")
    }
}

function Invoke-RewriteSuccess {
    $preflight = Invoke-Acf -Method POST -Path "/reviews/$ReviewReportId/rewrites/preflight" -Body @{
        selectedIssueIds = @($ReviewIssueId)
        optionalInstructions = 'scripted rewrite success'
        rewriteOptions = @{ strategy = 'targeted_fix' }
    }
    $run = Invoke-Acf -Method POST -Path "/reviews/$ReviewReportId/rewrites" -Body @{ preflightToken = $preflight.preflightToken } -IdempotencyKey 'acf-i19-script-rewrite-success'
    return [pscustomobject]@{
        Preflight = $preflight
        Run = (Wait-Run -RunId $run.id)
        Events = (Get-Events -RunId $run.id)
        Result = (Invoke-Acf -Method GET -Path "/workflow-runs/$($run.id)/rewrite-result")
    }
}

$review = Invoke-ReviewNeedsChanges
$rewrite = Invoke-RewriteSuccess

[ordered]@{
    review = @{
        runId = $review.Run.id
        status = $review.Run.status
        externalExecutionId = $review.Run.externalExecutionId
        eventCount = $review.Events.Count
        latestReportId = $review.Summary.latestReport.id
        latestConclusion = $review.Summary.latestReport.conclusion
    }
    rewrite = @{
        runId = $rewrite.Run.id
        status = $rewrite.Run.status
        externalExecutionId = $rewrite.Run.externalExecutionId
        eventCount = $rewrite.Events.Count
        candidateVersionId = $rewrite.Result.candidateVersion.id
        candidateIsCurrent = $rewrite.Result.candidateIsCurrent
    }
} | ConvertTo-Json -Depth 10
