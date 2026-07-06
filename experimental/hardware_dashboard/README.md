# Hardware Dashboard - Perf Builders & Testers

A modern, glassmorphic UI dashboard that seamlessly aggregates, visualizes, and monitors the status of Chromium performance testers and builders.

## Key Features Built

1. **Starlark Triggers Mapping**:
   - Dynamically parses the `testing/buildbot/chromium.perf.star` Starlark file via Gitiles to perfectly map tester bots back to their parent CI builders.
2. **GN Args Extraction**:
   - Fetches and parses `mb_config.pyl` via Python's AST evaluator, providing a neat togglable dropdown showing exactly what GN arguments each builder was compiled with.
3. **Swarming API Integration**:
   - Parses `chromium.perf.json` to extract precise swarming dimensions for every tester.
   - Fetches active status and bot counts using `luci-auth` authenticated calls to Swarming's gRPC/PrPC endpoints (both `chrome-swarming` and `chromeos-swarming`).
   - Uses an `AsyncQueue` to throttle requests and avoid 429 Rate Limiting from Swarming APIs.
   - Safely de-duplicates identical bots that are tagged as both Dead and Quarantined.
4. **Benchmarks Mapping (In-Memory Tarball Extraction)**:
   - Fetches the entire `tools/perf/core/schedule` Gitiles tarball into memory.
   - Parses the benchmark CSV files without writing them to disk to accurately determine which benchmarks run on which tester.
   - Formats benchmarks visually into clean wrap-around grid badges.
5. **Real-time Client-Side Search**:
   - A highly responsive top-right search bar that instantly filters out the DOM.
   - Searching matches against builder names, tester names, mapped dimensions, and all benchmark titles.
6. **One-Click Buganizer Escalation**:
   - For any tester that has Quarantined or Dead bots, a red "File Bug" button dynamically appears.
   - The button generates a massive URL payload that pre-fills a Buganizer `CUSTOMER_ISSUE` template with the bot hostnames, swarming pool, component `1735976`, auto-CC'd mailing lists, and priority statuses.
7. **Graceful Background Refresh**:
   - Every 5 minutes, the dashboard automatically syncs all configs and APIs in the background.
   - Shows a non-intrusive floating overlay and preserves any rows you've expanded and any search filters you're actively typing.
   - Loading checklists display elapsed millisecond time durations for every API phase.

## Prerequisites

- Python 3.7+
- `luci-auth` installed and authenticated on your PATH (used for Swarming RPC).
- Make sure to clear your browser cache if you modify `app.js` locally.

## Usage

Start the backend server:

```bash
# Recommended: Provide your local Chromium checkout directory for 10x faster startup
python3 server.py --chromium-dir /usr/local/google/home/jeffyoon/Projects/chromium/src

# Fallback: Start without directory (will fetch all files remotely via Gitiles)
python3 server.py
```

Then simply open your browser and navigate to:
[http://localhost:8000](http://localhost:8000)

## Technical Architecture

- **Backend (`server.py`)**: A lightweight `http.server.SimpleHTTPRequestHandler` with custom `/api/...` routes acting as a proxy and scraper to sidestep CORS limits.
- **Frontend (`app.js`, `index.html`, `styles.css`)**: Vanilla JavaScript manipulating a flexbox-heavy, dynamically generated HTML table with no external dependencies (No React/Vue required).
