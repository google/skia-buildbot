# Cloud Spanner Schema Migrations & Validation

This package manages database schema migrations and validations for the Performance Dashboard (perf) Cloud Spanner PostgreSQL database.

---

## SECTION 1: Architecture & Execution Flow

We use a **Sequential Versioned SQL Migrations** model to securely support delayed, batched, and zero-downtime deployments.

### 1. Sequence-Numbered SQL Scripts

All database schema upgrades are written as raw SQL DDL scripts inside the [migrations/](migrations/) directory. Each file is named with a four-digit zero-padded sequence number:

- `migrations/0001_init.sql`: Baseline schema containing all tables

### 2. Migration Logging Table

A tracking table named `schema_migrations` logs all successfully applied versions:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

### 3. Upgrade Execution Flow

On deployment, the background `maintenance` process runs `ValidateAndMigrateNewSchema`:

1. Creates the `schema_migrations` table if it doesn't exist.
2. Queries the maximum applied version from `schema_migrations`.
3. Evaluates all embedded scripts in `migrations/`, filtering out already applied versions.
4. Executes pending scripts chronologically and records them in `schema_migrations`.
5. Performs **Final Schema Catalog Verification**: Loads the expected schema layout from `schema_spanner.json` and validates it against the live catalog description using `assertdeep.Diff` to guarantee the schema was correctly upgraded.

### 4. Database Auto-Bootstrapping for Existing Instances

To ensure a seamless rollout without manual database log intervention, the runner has a built-in bootstrapping logic:

- If `schema_migrations` is empty, the runner checks if the database was already initialized:
  - If the `autobisections` table exists, it indicates the initial schema layout (version 1) has already been successfully set up.
  - The runner automatically bootstraps the history by inserting version `1` directly into the `schema_migrations` table.
  - If no database tables exist, the database is considered uninitialized (version 0) and all migrations are run from scratch.

### 5. Active Readiness Probe Gating (Zero-Downtime Deployments)

In production, instead of crash-looping or panicking when schemas are out of date, regular application containers (`frontend` and `ingest`) decouple database boot checks from startup:

- **Dynamic Environment Check**: On startup, the server checks the `KUBERNETES_SERVICE_HOST` environment variable:
  - **In Local Dev (K8s var is empty)**: The server fails-fast immediately on boot, giving the developer an instant, clear error trace so they know to run migrations locally.
  - **In Production (K8s var is present)**: Connection-level validation is bypassed on boot, allowing the container to start up successfully and delegating verification entirely to the readiness probe.
- **Readiness Gating**: The container exposes a `/readiness` HTTP endpoint. On every poll, the readiness handler checks `expectedschema.VerifySchemaVersion(ctx, f.db)`.
- **State Gating**: If the database version is lower than the expected binary version, `/readiness` returns `503 Service Unavailable`. Kubernetes holds the new container in the `NotReady` state (preventing any live traffic routing) while older containers continue serving traffic safely.
- **Background Logging**: A lightweight background thread logs a status update every 10 seconds (`Database schema is not ready yet... Waiting for migration job...`) until the migration job upgrades the database, at which point `/readiness` turns green (`200 OK`) and traffic seamlessly shifts.

---

## SECTION 2: Developer Guidelines: Adding a New Schema Change

When you need to modify the database schema, follow this step-by-step workflow:

### Step 1: Create the Migration File

- Add a new sequential file `migrations/000X_description.sql` under the `migrations/`
  folder (e.g. `0003_add_new_column.sql`).
- Add your DDL statements (e.g. `ALTER TABLE ... ADD COLUMN ...`).
- Place the manual rollback DDL inside a `-- Rollback SQL` comment block at the top of the file:

  ```sql
  -- Rollback SQL (For manual reference):
  -- ALTER TABLE Autobisections DROP COLUMN new_column;

  ALTER TABLE Autobisections ADD COLUMN new_column TEXT;
  ```

### Step 2: Update the Go Struct Definition

- Update the corresponding Go struct representations of your table (e.g., inside
  `perf/go/autobisection/sqlautobisectionstore/schema/schema.go` or other schemas)
  to match your new column or index.

### Step 3: Regenerate SQL Schema and Catalog JSON

Run the regeneration scripts inside `perf/` directory to automatically generate the expected SQL and JSON catalogs:

1. To regenerate Go schema code (`spanner/schema_spanner.go`), run:
   ```bash
   cd perf/go/sql && go run ./tosql
   ```
2. Start your local database emulator, and regenerate `schema_spanner.json` by running:
   ```bash
   cd perf && go run ./go/sql/exportschema --out ./go/sql/expectedschema/schema_spanner.json --databaseType spanner
   ```

### Step 4: Add & Run Unit Tests

- Verify that all migrations run and match perfectly by executing:
  ```bash
  bazelisk test //perf/go/sql/expectedschema/...
  ```
- Make sure to start the Spanner emulator container (`make -C perf run-spanner-emulator`)
  and set the printed environment variables before running unit tests.
