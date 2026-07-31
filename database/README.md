# ACF Database Management v2.6

## Important

Do not use the earlier package.

Version 2.2 makes database reset and fixture loading atomic:

1. `BEGIN`
2. dynamically truncate every `public` business table except `schema_migrations`
3. load the deterministic fixture
4. run fixture assertions
5. dynamically verify every business table is non-empty
6. `COMMIT`

Any SQL failure closes the `psql` session with the transaction uncommitted, so PostgreSQL rolls the reset back.

## Canonical layout

- `apps/api/migrations/`: immutable production Migration history; never deleted.
- `database/testdata/complete-test-data.sql`: deterministic complete fixture.
- `database/Initialize-Database.ps1`: backup, atomic clear, fixture load, and row-count report.
- `database/Consolidate-Database-Scripts.ps1`: dry-run/apply tool for centralizing old database scripts.
- `database/Test-Database-Management-Package.ps1`: local PowerShell Parser and tool preflight.

## Required environment

Run in Windows PowerShell from the repository root.

Required commands:

- `git`
- `psql`
- `pg_dump`

Set the real local connection:

```powershell
$env:DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:15433/ai_content_factory?sslmode=disable'
```

## Execution order

### 1. Extract package into repository root

After extraction, the repository contains:

```text
database/
  Initialize-Database.ps1
  Consolidate-Database-Scripts.ps1
  Test-Database-Management-Package.ps1
  testdata/complete-test-data.sql
```

### 2. Run package preflight

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\database\Test-Database-Management-Package.ps1
```

Do not continue unless it prints `PASS`.

### 3. Preview database-script consolidation

This does not delete or move files:

```powershell
.\database\Consolidate-Database-Scripts.ps1
```

Review:

```powershell
Import-Csv .\database\reports\database-script-consolidation-plan.csv |
    Format-Table -AutoSize
```

Do not apply consolidation yet.

### 4. Atomically reset and seed the database

Default mode creates a data-only backup outside the repository:

```powershell
.\database\Initialize-Database.ps1 `
    -ConfirmClearData `
    -Confirm:$false
```

Only when an external backup is definitely unnecessary:

```powershell
.\database\Initialize-Database.ps1 `
    -ConfirmClearData `
    -SkipBackup `
    -Confirm:$false
```

The script refuses databases other than `ai_content_factory` and requires the database's applied Migration version set to exactly match the repository's `*.up.sql` version set.

### 5. Run existing consistency validation

Before script consolidation, use the current path:

```powershell
powershell -ExecutionPolicy Bypass `
    -File .\scripts\validate-database-consistency.ps1
```

Expected result:

- Schema checks: `53/53`
- Data checks: `45/45`
- final `PASS`

If validation fails, do not apply consolidation and do not submit dbcheck.

### 6. Apply database-script consolidation

After reset and consistency validation both pass:

```powershell
.\database\Consolidate-Database-Scripts.ps1 `
    -Apply `
    -Confirm:$false
```

Unknown maintenance scripts are moved to `database/legacy-imported/` for review. They are not deleted unless `-DeleteUnknownMaintenance` is also supplied.

### 7. Review Git changes

```powershell
git status --short
git diff --stat
git diff
```

The consolidation script does not create a Commit.

## Standalone fixture loading

Use only when all business tables are already empty:

```powershell
psql $env:DATABASE_URL `
    -X `
    --no-psqlrc `
    --set=ON_ERROR_STOP=1 `
    --file .\database\testdata\complete-test-data.sql
```


## V2.3 consolidation safety

The consolidation tool uses an explicit allowlist and deletes nothing.

It intentionally preserves the Iteration 06 QA suite, production verification,
browser acceptance reports, infrastructure startup, repository scaffolding,
review-bundle tooling, and Iteration 17/18 contract validators.


## V2.4 fully self-validating consolidation

Manual output review is no longer required.

Run:

```powershell
.\database\Consolidate-Database-Scripts.ps1 -Apply -Confirm:$false
```

The script validates the exact eight-file allowlist, all protected files,
destination conflicts, canonical Migration history, reference updates, stale
references, and `git diff --check`. Any failed condition returns `BLOCKED`.


## V2.5 execution model

Use the single Downloads initializer:

`Initialize-ACF-Database-Maintenance.ps1`

It installs this package, parses every PowerShell script, optionally resets the
database, applies the exact consolidation allowlist, runs database consistency
validation, and returns only PASS or BLOCKED as the final state.


## V2.6 partial-run recovery

The consolidator updates path references only after Git moves are complete.
It supports both initial and partially completed states and is idempotent.
