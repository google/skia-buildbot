# Trace Visibility & Promotion Architecture

Manages trace visibility rules (e.g., determining which traces/bots are public
vs. internal) using a `Checker` background loop and a standalone `perf-tool`
promotion utility.

---

## 1. Unified Database Architecture

To streamline detection and analysis, both public and internal frontends share
a single database. Traces are marked as public or internal via the `is_public`
column in the `TraceParams` table.

```text
               +--------------------------------------------+
               |              Unified Database              |
               |             (Spanner)                      |
               +--------------------------------------------+
                                      |
                      +---------------+---------------+
                      |                               |
       is_public = TRUE                               is_public = FALSE/NULL
                      v                               v
         +--------------------------+    +--------------------------+
         |     Public Frontend      |    |    Internal Frontend     |
         |    (perf.luci.app)       |    |  (chrome-perf.corp.goog) |
         +--------------------------+    +--------------------------+
         | Only loads public traces |    | Loads ALL traces into    |
         | into InMemoryTraceParams |    | memory cache             |
         +--------------------------+    +--------------------------+
```

---

## 2. Dynamic Promotion Components

To promote large historical private trace sets without triggering full table
scans on millions of records, the architecture uses a synced hourly `Checker`
loop in the background and a robust standalone `perf-tool` command for
promotion sweeps.

```text
                                    +----------------------------------------+
                                    |              Provider                  |
     +-------------------+          | (e.g., ChromeProvider)                 |
     |     Database      |          |                                        |
     | (PublicTraceRules)|          | 1. Connect to Config Sources           |
     +-------------------+          |    (e.g., Gitiles repo, Gerrit)        |
             ^                      | 2. Parse domain-specific configs       |
             | (1) Sync             |    (e.g., public_builders.json)        |
             v                      | 3. Format as standard rule expressions |
     +-------------------+          |    (e.g., "bot=Mac", "bot=Linux")      |
     |                   |          +----------------------------------------+
     |      Checker      |                                |
     | (Hourly singleton)|  (2) GetExpectedRules()        |
     |  - Compare Rules  |<-------------------------------+
     |  - Sync DB rules  |
     +-------------------+
             |
             | (3) Updates rules table
             v
     +-------------------+
     |     Database      |
     | (PublicTraceRules)|
     +-------------------+
             ^
             | (4) Run manually or via cron/orchestrator
             v
     +-------------------+
     |     perf-tool     |
     |  (CLI promotion)  |
     +-------------------+
             |
             | (5) Direct index-free JSONB queries & safe updates
             v
     +-------------------+
     |    TraceParams    |
     |  - Promote new    |
     +-------------------+
```

### 2.1 The Checker (`perf/go/trace_visibility/checker`)

An hourly background routine running in the maintenance daemon that:

- Queries a `Provider` (e.g., `ChromeProvider` reading builder configs from
  Gitiles) for expected public rules.
- Syncs these rules to the local `PublicTraceRules` table.

### 2.2 The Promoter (`perf/go/trace_visibility/promoter`)

A stateful batch promotion utility executed via the
`perf-tool visibility promote` CLI command that:

- Pulls active public rules from the database.
- Queries private traces matching those rules and batch promotes them to public
  (`is_public = true`) state using fast parallel chunk updates.

### 2.3 The Provider (`perf/go/trace_visibility/provider.Provider`)

Retrieves domain-specific configurations (like raw builder lists from
repository files) and formats them into standard rule tags (e.g.,
`bot=android-pixel6-perf`).

---

## 3. Configuration

The visibility components are initialized when `visibility_config` is
defined in the instance's configuration:

```json
"visibility_config": {
    "provider_name": "chrome",
    "sources": {
        "chrome_bot": {
            "git_repo": "https://chromium.googlesource.com/chromium/src",
            "path": "tools/perf/public_builders.json",
            "rule_prefix": "bot="
        }
    }
}
```

If `visibility_config` is omitted, no checkers or promoters are run.
