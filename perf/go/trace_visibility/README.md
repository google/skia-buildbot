# Trace Visibility

Manages trace visibility rules (e.g., determining which traces/bots are public vs. internal) using a `Checker -> Provider` architecture.

## Architecture Overview

```text
                               +----------------------------------------+
                               |              Provider                  |
+-------------------+          | (e.g., ChromeProvider)                 |
|     Database      |          |                                        |
| (PublicTraceRules)|          | 1. Connect to Config Sources           |
+-------------------+          |    (e.g., Gitiles repo, Gerrit)        |
        ^                      | 2. Parse domain-specific configs       |
        | (1) Read             |    (e.g., public_builders.json)        |
        v                      | 3. Format as standard rule expressions |
+-------------------+          |    (e.g., "bot=Mac", "bot=Linux")      |
|                   |          +----------------------------------------+
|      Checker      |                                |
|                   |  (2) GetExpectedRules()        |
|  - Compare Rules  |<-------------------------------+
|  - Extract Source |
|    (e.g., "bot=") |
|  - Group Diffs    |
+-------------------+
        | (3) Emit
        v
+-------------------+
|  Logs & Metrics   |
| (per source tag)  |
+-------------------+
```

### Components

- **Checker (`perf/go/trace_visibility/checker`)**: The data-agnostic orchestrator responsible for verification.

  - **Responsibilities**:
    - Fetches the current set of visibility rules stored in the SQL database.
    - Queries the registered `Provider` to get the actual _expected_ rules.
    - Compares the two sets to find discrepancies (missing in DB, or extra in DB).
    - Parses the rule expressions to extract the "source" (typically the rule prefix, such as `bot=`).
    - Groups the discrepancies by their source and emits tagged metrics (`perf_visibility_rules_diff`) and alerts/logs per source.

- **Provider (`perf/go/trace_visibility/provider.Provider`)**: Encapsulates the domain-specific logic required to determine what the visibility rules _should_ be.
  - **Responsibilities**:
    - Understands how to interact with external systems (like Gitiles, a custom API, etc.) to fetch raw configuration files.
    - Parses these configurations and translates them from their domain-specific format into standard database rule expressions (e.g., converting a list of public builder names into `["bot=Builder1", "bot=Builder2"]`).
    - Returns a flat map of these expected rules to the `Checker`.

## Configuration

The visibility provider is instantiated in `maintenance.go` based on the `visibility_config` object in the instance's JSON configuration. It specifies the `provider_name` and contains a map of named sources.

For example:

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

If the `visibility_config` object is missing, no provider is instantiated.
