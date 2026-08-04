package ready

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var migrationFilePattern = regexp.MustCompile(`^(\d+)_.+\.(up|down)\.sql$`)

// Checker evaluates /readyz critical dependency checks without mutating state.
type Checker struct {
	Pool                  *pgxpool.Pool
	ExpectedMigrationHead int
	Worker                *workflowrun.WorkerHealth
	WorkerMaxIdle         time.Duration
	Now                   func() time.Time
}

// Result is a controlled readiness outcome suitable for HTTP responses.
type Result struct {
	Ready  bool
	Checks map[string]string
}

// Check runs critical readiness probes. It never returns secrets or SQL text.
func (c *Checker) Check(ctx context.Context) Result {
	checks := map[string]string{
		"api":       "ok",
		"database":  "fail",
		"migration": "fail",
		"worker":    "fail",
	}
	if c == nil {
		return Result{Ready: false, Checks: checks}
	}
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	maxIdle := c.WorkerMaxIdle
	if maxIdle <= 0 {
		maxIdle = 2 * time.Minute
	}

	if c.Pool == nil {
		checks["database"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.Pool.Ping(pingCtx); err != nil {
		checks["database"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	var one int
	if err := c.Pool.QueryRow(pingCtx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		checks["database"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	checks["database"] = "ok"

	version, dirty, err := readMigrationState(pingCtx, c.Pool)
	if err != nil {
		checks["migration"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	if dirty {
		checks["migration"] = "dirty"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	if c.ExpectedMigrationHead < 1 {
		checks["migration"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	if version < c.ExpectedMigrationHead {
		checks["migration"] = "behind"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	if version > c.ExpectedMigrationHead {
		checks["migration"] = "ahead"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	checks["migration"] = "ok"

	if c.Worker == nil || !c.Worker.Healthy(now, maxIdle) {
		checks["worker"] = "unavailable"
		return Result{Ready: false, Checks: withReady(checks)}
	}
	checks["worker"] = "ok"
	return Result{Ready: true, Checks: withReady(checks)}
}

func withReady(checks map[string]string) map[string]string {
	out := make(map[string]string, len(checks)+1)
	for k, v := range checks {
		out[k] = v
	}
	ready := true
	for _, v := range checks {
		if v != "ok" {
			ready = false
			break
		}
	}
	if ready {
		out["ready"] = "ok"
	} else {
		out["ready"] = "fail"
	}
	return out
}

func readMigrationState(ctx context.Context, pool *pgxpool.Pool) (version int, dirty bool, err error) {
	var exists bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = 'schema_migrations'
	)`).Scan(&exists); err != nil {
		return 0, false, err
	}
	if !exists {
		return 0, false, fmt.Errorf("schema_migrations missing")
	}
	if err = pool.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return 0, false, err
	}
	if version > 0 {
		var count, minVersion int
		if err = pool.QueryRow(ctx, "SELECT COUNT(*), MIN(version) FROM schema_migrations").Scan(&count, &minVersion); err != nil {
			return 0, false, err
		}
		if count != version || minVersion != 1 {
			dirty = true
		}
	}
	return version, dirty, nil
}

// DiscoverMigrationHead returns the highest migration version present on disk.
// It never executes migrations and must not hardcode a version number.
func DiscoverMigrationHead(directory string) (int, error) {
	if strings.TrimSpace(directory) == "" {
		directory = os.Getenv("MIGRATIONS_DIR")
	}
	if strings.TrimSpace(directory) == "" {
		directory = "migrations"
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, err
	}
	versions := map[int]struct{ up, down bool }{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		state := versions[version]
		if matches[2] == "up" {
			state.up = true
		} else {
			state.down = true
		}
		versions[version] = state
	}
	if len(versions) == 0 {
		return 0, fmt.Errorf("no migrations in %s", filepath.Clean(directory))
	}
	ordered := make([]int, 0, len(versions))
	for version, state := range versions {
		if !state.up || !state.down {
			return 0, fmt.Errorf("migration %06d incomplete", version)
		}
		ordered = append(ordered, version)
	}
	sort.Ints(ordered)
	return ordered[len(ordered)-1], nil
}
