# Tool Fix and Self-Test Report

## Lightweight scene switcher
- BASELINE does not TRUNCATE full DB / re-seed / restart API
- Same scene second call returns PASS unchanged quickly
- Normal scenes apply 00-RESET + scene SQL only
- Empty scenes backup tables before truncate; restore on leave
- No pg_sleep(43200) row locks
- statement_timeout / lock_timeout enabled
- CANDIDATE_STALE fixed: base_chapter_version>=1, verified stale_conflict

## Capture runner CLI
- --only / --resume / --no-commit / --audit-only
- Drawer/dialog structural asserts
- Fresh screenshot mtime + SHA-256
- Atomic/safe writes

## Self-test results
- BASELINE x2 fast path: PASS
- Sample + queue recapture: completed
- UI-051: COLLECTED_TARGET
- Fixture Test: PASS after recapture
- API readyz: healthy during lightweight switches

## Constraints honored
- No full seed re-import
- No DB backup restore
- No product code / prototype changes
- No Mock
