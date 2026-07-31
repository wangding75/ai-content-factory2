# Unified P0 test data (09.F6E)

`09-f6e-unified-p0.ps1` manages the deterministic local P0 dataset. It targets only the repository's local Compose PostgreSQL service and refuses to write unless the compose project is `ai-content-factory2` and the connected database is `ai_content_factory`.

```powershell
.\database\legacy\test-data\09-f6e-unified-p0.ps1 -Action Load
.\database\legacy\test-data\09-f6e-unified-p0.ps1 -Action Verify
.\database\legacy\test-data\09-f6e-unified-p0.ps1 -Action Clean
.\database\legacy\test-data\09-f6e-unified-p0.ps1 -Action Exercise
```

`Exercise` checks first load, repeat load, SQL constraints, real API reads, namespaced cleanup, ordinary-data protection, and reload after cleanup. The SQL deletes only stable `f6e...` fixture IDs; it never truncates tables, clears the database, or removes volumes.
