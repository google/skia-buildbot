# Skia Perf Multigraph State Architecture: Legacy (`explore-multi-sk`) vs. V2 (`explore-multi-v2-sk`)

**Author:** L10 Principal Engineer (Perf) — _L11 Architecture Review & User Demo Readiness Brief_
**Scope:** [`explore-multi-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts), [`explore-simple-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts), [`dataframe_context.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts), [`test-picker-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts), [`plot-google-chart-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-google-chart-sk/plot-google-chart-sk.ts), [`plot-summary-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-summary-sk/plot-summary-sk.ts) vs. [`explore-multi-v2-sk.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts) & V2 sub-components.

---

## 1. Executive Verdict

> [!IMPORTANT]
> **1. Is the rumor true that `explore-multi-sk`, `explore-simple-sk`, sub-components, and `dataframe-repository-sk` each maintain overlapping, easily desynchronized state?**
> **Yes — 100% confirmed, and the reality in the code is worse than the rumor.** Legacy Multigraph distributes overlapping mutable state across **6 distinct component layers** and **3 separate copies of the DataFrame/DataTable per chart**, stitched together via imperative DOM mutations, bidirectional `CustomEvent` ping-pong, `setTimeout(..., 0)` yields, and `requestAnimationFrame` spin-locks.
>
> **2. Is `explore-multi-v2-sk` our salvation for user's demo tomorrow?**
> **Yes.** [`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L67-L3002) eliminates the entire class of structural state desynchronization bugs by collapsing state into a single reactive `LitElement` root, moving trace filtering to an off-main-thread Web Worker + WASM engine, replacing `explore-simple-sk` with stateless HTML5 `<canvas>` projections, and enforcing monotonic request IDs (`_latestRequestId`). However, V2 has **4 specific edge cases** you must either avoid during the demo or patch in ~20 lines of code today (detailed in [Section 6](#6-user-demo-playbook-4-sharp-edges-remaining-in-v2)).

---

## 2. Architectural Topology: 6-Layer Imperative Web vs. Unidirectional Reactive Tree

### Legacy Multigraph (`explore-multi-sk`) State Topology

In Legacy Multigraph, [`ExploreSimpleSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L387-L3923) was originally built as a standalone full-page monolith (`ElementSk`) with its own URL state, query builder, data fetcher, and selection state. [`ExploreMultiSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L122-L2211) was bolted on top to instantiate $N$ `ExploreSimpleSk` instances—using `exploreElements[0]` as a hidden (`display: none`) "master accumulator graph" when splitting—while [`DataFrameRepository`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L136-L526) (`LitElement` context provider) was later wedged inside each `ExploreSimpleSk`.

```mermaid
flowchart TD
    URL["Browser URL (?shortcut=..., begin, end, splitByKeys)"]

    subgraph L1["Layer 1: ExploreMultiSk (ElementSk)"]
        EMState["_state: State (begin, end, shortcut, splitByKeys, domain, pageOffset)"]
        EMArrays["5 Parallel Mutable Arrays:<br/>allGraphConfigs[] | allFrameRequests[] | allFrameResponses[]<br/>exploreElements[] | currentPageExploreElements[]"]
        HiddenE0["exploreElements[0] (Hidden Master Accumulator when split)"]
    end

    subgraph L2["Layer 2: TestPickerSk & TestPickerStateController"]
        TPState["fieldData[] | count | autoAddTrace | activeCascadeId"]
        TPDomQuery["Global DOM Queries:<br/>document.querySelector('explore-multi-sk')<br/>window.addEventListener('data-loaded')"]
    end

    subgraph L3["Layer 3: N x ExploreSimpleSk (ElementSk)"]
        ESState["_state: State (begin, end, queries, formulas, keys, selected, doNotQueryData)"]
        ESDF["_dataframe: DataFrame (Sliced copy of traceset/header/paramset)"]
        ESMutex["_requestId = 'foo' (Single-slot 409 Conflict Mutex)"]
    end

    subgraph L4["Layer 4: N x DataFrameRepository (LitElement Context)"]
        DFRState["_traceset | _header | _paramset | _baseRequest (Mutated in-place!)"]
        DFRTable["@provide data: google.visualization.DataTable"]
    end

    subgraph L5["Layer 5: N x PlotGoogleChartSk (LitElement)"]
        PGCState["selectedRange | domain | zoomedVRange | removedLabelsCache"]
        GChart["Async google.visualization.CoreChartBase + DataView"]
    end

    subgraph L6["Layer 6: N x PlotSummarySk (LitElement)"]
        PSState["selection (domain range) vs. selectionCoords (pixel range)"]
        PSChart["Downsampled DataTable + Async Google Chart"]
    end

    URL <-->|"stateReflector"| EMState
    EMState <-->|"Imperative setter + Events"| ESState
    EMArrays <-->|"Manual .splice() / _reindexGraphs()"| ESState
    HiddenE0 -.->|"rAF & setTimeout polling loops"| EMArrays
    L2 <-->|"plot-button-clicked / remove-trace / populate-query"| L1
    TPDomQuery -.->|"Reaches across DOM"| L1
    ESState <-->|"resetWithDataframeAndRequest()"| DFRState
    ESDF <-->|"Duplicate sliced state"| DFRState
    DFRTable -->|"Lit @consume"| PGCState
    DFRTable -->|"Lit @consume"| PSState
    PSState <-->|"summary_selected / SelectRange()"| ESState
    PGCState <-->|"toggleSelectionRange()"| ESState
```

### Multigraph V2 (`explore-multi-v2-sk`) State Topology

[`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L67-L3002) replaces all 6 legacy layers with a **single reactive `LitElement` state container**, off-main-thread WASM indexing, IndexedDB content-addressable caching, and **pure functional projection** during `render()`.

```mermaid
flowchart TD
    URL2["Browser URL (?shortcut=..., begin, end, splitKeys, diffBase)"]

    subgraph V2Root["Single Source of Truth: ExploreMultiV2Sk (LitElement)"]
        V2State["@state / @property:<br/>queries[] | _formulasPerQuery[] | splitKeys: Set | _seriesData: TraceSeries[]<br/>viewportMinX/MaxX | _globalHoverX | _globalPinnedX | _loadedBounds"]
        V2Concurrency["Concurrency Guards:<br/>_latestRequestId (Monotonic Counter)<br/>_prefetchAbortController (AbortController)<br/>_latestSuggestRequestIds[]"]
        V2Project["Pure Render-Time Projection (Zero Refetch / Zero DOM Tearing):<br/>computeTraceDiffs() -> computeCustomTransforms() -> computeSplitGroups()"]
    end

    subgraph OffThread["Off-Main-Thread & Persistence Layer"]
        WASM["ExploreWorkerController<br/>(Web Worker + filter.wasm + traces.bin)"]
        IDB["TraceDatabase (IndexedDB)<br/>SHA-256 hashRequest(req) Cache"]
    end

    subgraph V2Children["Controlled Stateless / Pure UI Children (Props Down, Intent Events Up)"]
        Toolbar["ExploreToolbarSk<br/>(@control-change, @split, @reset-zoom)"]
        QueryBar["M x QueryBarSk<br/>(@add-query, @remove-query, @pipeline-change)"]
        SummaryV2["PlotSummaryV2Sk<br/>(Sync HTML5 Canvas + Min-Max Decimation)"]
        ChartsV2["K x TraceChartSk<br/>(Sync Dual-Layer HTML5 Canvas: Background + Overlay)"]
    end

    URL2 <-->|"stateReflector"| V2State
    V2State -->|"postMessage (query, requestId)"| WASM
    WASM -->|"matchingIds + facet counts (guarded by requestId)"| V2State
    V2State <-->|"getFrameResponse / getTraceValues"| IDB
    V2State --> V2Project
    V2Project -->|".series, .viewportMinX/MaxX, .globalHoverX"| ChartsV2
    V2State -->|".allSeries, .viewportMinX/MaxX"| SummaryV2
    V2State -->|".query, .availableParams, .optionsByKey"| QueryBar
    V2State -->|".splitKeys, .dateMode, .evenXAxisSpacing"| Toolbar
    ChartsV2 -->|"Intent: @viewport-changed, @hover-changed, @pin-point"| V2State
    SummaryV2 -->|"Intent: @summary-selected, @load-range"| V2State
    QueryBar -->|"Intent: @add-query, @remove-query, @split"| V2State
    Toolbar -->|"Intent: @control-change, @split"| V2State
```

---

## 3. Forensic Proof: Why Legacy Multigraph Desyncs Constantly

Below are the exact mechanisms—with file and line references—showing how state fractures across [`ExploreMultiSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts), [`ExploreSimpleSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts), [`DataFrameRepository`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts), [`TestPickerSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts), [`PlotGoogleChartSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-google-chart-sk/plot-google-chart-sk.ts), and [`PlotSummarySk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-summary-sk/plot-summary-sk.ts).

### 3.1. Five Parallel Arrays & The Hidden `exploreElements[0]` Accumulator Hack

[`ExploreMultiSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L122-L135) does not store a normalized table of traces. Instead, it maintains **five separate mutable arrays** that must remain index-aligned at all times:

1. [`allGraphConfigs: GraphConfig[]`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L123) (the queries/formulas/keys per graph)
2. [`allFrameResponses: FrameResponse[]`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L125) (cached backend responses per graph)
3. [`allFrameRequests: FrameRequest[]`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L127) (cached requests per graph)
4. [`exploreElements: ExploreSimpleSk[]`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L131) (the full pool of `ExploreSimpleSk` DOM elements)
5. [`currentPageExploreElements: ExploreSimpleSk[]`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L133) (the currently visible page slice)

When `state.splitByKeys.length > 0` and multiple graphs exist, [`ExploreMultiSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L1596-L1616) hides `exploreElements[0]` (`elem.style.display = 'none'`) and uses it as a **hidden master accumulator graph**, shifting all real split graphs to indices `1..N`. Every add, remove, split, or pagination operation has to manually `.splice()`, `.unshift()`, `.pop()`, and call [`_reindexGraphs()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L982-L993) depending on whether `exploreElements.length > 1 && state.splitByKeys.length > 0`.

Worse, to populate split graphs on load or trace addition, [`ExploreMultiSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts) tells `exploreElements[0]` to fetch data and then **busy-waits via `requestAnimationFrame` and `setTimeout` polling loops** waiting for `exploreElements[0].spinning` and `exploreElements[0].dataLoading` to become `false`:

- [`_onPlotButtonClicked`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L334-L338) & [`_onAddToGraph`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L414-L426):
  ```ts
  while (explore0.spinning || explore0.dataLoading) {
    await new Promise((resolve) => requestAnimationFrame(resolve));
  }
  await this.splitGraphs(false, true);
  ```
- [`checkDataLoaded`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L1457-L1475): Polls `mainGraph.spinning` via `setTimeout(() => this.checkDataLoaded(), 100)` up to 500 retries (50 seconds) before calling `splitGraphs()`.

### 3.2. The 275-Line `_onRemoveTrace` Race Condition Minefield

Look at [`ExploreMultiSk._onRemoveTrace`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L598-L873) to see how badly these layers collide when a user removes a parameter value from the test picker:

1. **Concurrent Event Mutation ([`L611-L618`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L611-L618)):** Removing the last split value dispatches both `remove-trace` and `split-by-changed`. Because `_onRemoveTrace` yields via `await new Promise((resolve) => setTimeout(resolve, 0))`, `_onSplitByChanged` runs during the `setTimeout` and wipes `this.state.splitByKeys = []` mid-function! The code tries to survive this by snapshotting `const currentSplitByKeys = [...this.state.splitByKeys]` before the `setTimeout`.
2. **Guessing Which Copy of `TraceSet` Is Less Stale ([`L756-L778`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L756-L778)):** When rebuilding the remaining graphs, `ExploreMultiSk` compares `this.allFrameResponses[0].dataframe.traceset` against `this.exploreElements[0].getTraceset()` and picks whichever object has more keys:
   ```ts
   // Look for traces in exploreElements[0] first, as it may have more up-to-date data
   // (e.g. from extended range) than allFrameResponses[0].
   const elemTraceset = this.exploreElements[0]?.getTraceset();
   if (elemTraceset && Object.keys(elemTraceset).length > Object.keys(traceSet).length) {
     traceSet = elemTraceset as TraceSet;
   }
   ```
3. **Order-Dependent State Overwrites ([`L849-L865`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L849-L865)):** An explicit inline comment admits that calling `removeExplore(elemsToRemove)` triggers `renderCurrentPage()`, which reads `this.allGraphConfigs[i].queries` and **clobbers** `elem.state.queries` if `elem.state` isn't mutated in the exact right sequence with `doNotQueryData = true`.

### 3.3. Three Copies of the DataFrame per Graph & In-Place Request Mutation

Every single [`ExploreSimpleSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts) graph on the page maintains **three separate representations** of the same trace data:

1. **`ExploreMultiSk.allFrameResponses[i]`** ([`explore-multi-sk.ts#L125`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L125)): The raw `FrameResponse` snapshot at split/load time (never updated when an individual chart pans or zooms).
2. **`DataFrameRepository._traceset` / `_header` / `dataframe` / `data` (`DataTable`)** ([`dataframe_context.ts#L140-L167`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L140-L167)): Holds the full loaded range for that single chart.
   - Its [`resetWithDataframeAndRequest`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L423-L444) method executes `this._baseRequest.request_type = 0` ([`L430`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L430)), **mutating the shared `FrameRequest` reference passed in from `ExploreMultiSk.allFrameRequests[i]` in place**.
   - Its [`addTraceInfo`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L230-L298) only knows how to prepend before `header[0]` or append after `header[last]` ([`L240-L249`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L240-L249)) and silently slices off data exceeding `MAX_DATAPOINTS = 20000` from the front ([`L262-L271`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L262-L271)).
   - Worse, [`addTraceInfo`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/dataframe/dataframe_context.ts#L277-L296) contains a logic bug in its `traceMetadata` merge loop: `if (!exists)` is placed _inside_ the `for (const existing of this._traceMetadata)` loop, causing metadata to be duplicated on every non-matching iteration!
3. **`ExploreSimpleSk._dataframe`** ([`explore-simple-sk.ts#L415-L421`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L415-L421)): A _sliced_ sub-DataFrame mutated inside [`updateSelectedRangeWithUpdatedDataframe`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L2159-L2193) to match the current zoom window.

### 3.4. Side-Effect-Laden `set state()` Setter & The `_requestId = 'foo'` 409 Mutex

In [`ExploreSimpleSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L3735-L3805), assigning `exploreSimpleSk.state = newState` is not a passive property update—it is a 70-line procedural trigger that:

- Calls [`rationalizeTimeRange(state)`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L2546-L2585), which can silently rewrite `state.begin` and `state.end` if `end - begin < 100`.
- Mutates child DOM widgets (`this.range.state`, `this.summary.selected`, `this.commitsTab`).
- Fires [`updateTestPickerUrl()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L2517-L2544) (which issues async `updateShortcut` POSTs).
- Triggers [`rangeChangeImpl()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L3802-L3804) to fetch backend data _unless_ the caller remembered to set `state.doNotQueryData = true` before assignment.

Furthermore, [`ExploreSimpleSk.requestFrame`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-simple-sk/explore-simple-sk.ts#L3668-L3695) guards all network fetches with a single string mutex:

```ts
if (this._requestId !== '') {
  return Promise.reject(new RequestError('There is a pending query already running.', 409));
}
this._requestId = 'foo';
```

Instead of canceling or superseding in-flight requests when a user clicks quickly, `ExploreSimpleSk` **rejects the new user action with HTTP 409**, leaving the UI state reflecting the _new_ click while the chart renders the _old_ in-flight request!

### 3.5. Sub-Component Desyncs (`TestPickerSk`, `PlotGoogleChartSk`, `PlotSummarySk`)

- **`TestPickerSk` Global DOM Peeking:** [`TestPickerSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L152-L185) and [`TestPickerStateController`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-state-controller.ts#L40-L63) maintain their own `fieldData[]`, `count`, `autoAddTrace`, and `dataLoading` flags. Because state isn't passed down reactively, [`TestPickerSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L178) literally executes `document.querySelector('explore-multi-sk')` at lines [`L178`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L178), [`L307`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L307), and [`L455`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L455) to inspect `exploreMulti._ dataLoading` and `exploreMulti.exploreElements.length`, and listens to global `window.addEventListener('data-loaded', ...)` ([`L155`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-sk.ts#L155)).
- **`PlotGoogleChartSk` & `PlotSummarySk` Domain Toggle Desync:** [`PlotGoogleChartSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-google-chart-sk/plot-google-chart-sk.ts#L465-L474) has an explicit comment on `toggleSelectionRange()` admitting the architecture is broken when switching between `commit` and `date` domains:
  ```ts
  // Note, the way this function works is imperfect. The ideal way to do this is for
  // explore-simple-sk to be aware of the selection range from plot-summary-sk...
  // Since that takes a larger refactoring, this is a quick way to fix the zooming bug.
  // TODO(b/362831653): Fix frame shifting from toggling domain
  ```
- **Google Charts Async Ready Race in `PlotSummarySk`:** [`PlotSummarySk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-summary-sk/plot-summary-sk.ts#L80-L222) stores both `selection: range | null` (domain coordinates) and `selectionCoords: range | null` (pixel coordinates), and can only convert between them _after_ the underlying `google-chart` fires its asynchronous `google-chart-ready` event ([`L315-L334`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/plot-summary-sk/plot-summary-sk.ts#L315-L334)). If `selectedValueRange` is set while Google Charts is still laying out, `selection` and `selectionCoords` desync until the next ready callback.

---

## 4. How `explore-multi-v2-sk` Solves the Problem Architecturally

[`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L67-L3002) was engineered specifically to eliminate every failure mode above.

### 4.1. Single Source of Truth & Pure Render-Time Projection

In [`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L78-L135), all application state lives on the root `LitElement`:

- **Query & Split State:** `queries: Record<string, string[]>[]`, `_formulasPerQuery: string[][]`, `splitKeys: Set<string>`, `splitAll: boolean`, `_diffBase: { key: string; val: string } | null`, `_transformPreset: string`.
- **Canonical Data Store:** `_seriesData: TraceSeries[]` ([`L105`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L105)) holds all fetched trace series once. There is no hidden `exploreElements[0]`, no `DataFrameRepository`, and no Google `DataTable`.
- **Viewport & Crosshair State:** `viewportMinX: number | null`, `viewportMaxX: number | null`, `_globalHoverX: number | null`, `_globalPinnedX: number | null`, `_loadedBounds`, `_globalBounds`.

Crucially, **splitting, diffing, normalizing, and pagination do not mutate state arrays or re-fetch from the backend**. Inside [`ExploreMultiV2Sk.render()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L2870-L2917), the chart groups are computed as a **pure functional pipeline** over `this._seriesData`:

```ts
let displaySeries = computeTraceDiffs(this._seriesData, this._diffBase);
displaySeries = computeCustomTransforms(displaySeries, this._transformPreset);
// ... filter & paginate to currentPageTraces ...
const groups = computeSplitGroups(currentPageTraces, this.splitKeys, this.splitAll);
```

When an user clicks a "Split by" pill in [`QueryBarSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/query-bar-sk.ts) or [`ExploreToolbarSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-toolbar-sk.ts), [`_toggleSplitKey`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1082-L1094) updates `this.splitKeys = new Set(...)` and Lit synchronously re-projects `groups` in <2ms without touching the network or ripping apart stateful child components.

### 4.2. Monotonic Request Sequencing & Cancellation (Zero Stale Overwrites)

Instead of `ExploreSimpleSk`'s `_requestId = 'foo'` mutex that throws `409 Conflict`, [`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L319) uses a monotonically increasing counter `_latestRequestId` across the entire async pipeline:

1. [`_onQueriesChanged`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1055) increments `const requestId = ++this._latestRequestId` and passes `requestId` into `_triggerWorkerFilter(requestId)`.
2. When the Web Worker returns matching trace IDs, [`_fetchData(traceIds, requestId)`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1375-L1377) checks `if (requestId !== this._latestRequestId) return;` before and after every `await` ([`L1425`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1425), [`L1443`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1443)).
3. Even if an older fetch completes late, V2 still writes the response into IndexedDB (`this._db.setFrameResponse(reqHash, json)`) so the work isn't wasted, and then cleanly exits (`if (requestId !== this._latestRequestId) return;`) without touching UI state!
4. Background history prefetching ([`_prefetchHistory`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1517-L1526)) uses `this._prefetchAbortController = new AbortController()` to immediately abort stale background fetches when queries change.

### 4.3. Off-Main-Thread WASM Param Indexing (`ExploreWorkerController`)

Legacy [`TestPickerStateController`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/test-picker-sk/test-picker-state-controller.ts#L221-L273) fires sequential `/_/nextParamList/` HTTP POST requests every time a user selects a dropdown value, causing network waterfalls and race conditions (`activeCascadeId`).

V2's [`ExploreWorkerController`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-worker-controller.ts#L29-L266) loads a compact inverted index (`params.json` + `traces.bin`) into a Web Worker backed by `filter.wasm` (with a JS fallback). Every query change runs [`_triggerWorkerFilter`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L649-L669) locally off the main thread in milliseconds, returning both the exact `matchingParams` and leave-one-out facet counts (`countsByQuery`) in a single atomic message guarded by `requestId`.

### 4.4. Synchronous Dual-Layer HTML5 Canvas (`TraceChartSk` & `PlotSummaryV2Sk`)

V2 completely removes Google Charts (`@google-web-components/google-chart`) and `DataTable`:

- [`TraceChartSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/trace-chart-sk.ts#L92-L1200) renders onto two stacked `<canvas>` elements: a background `#chart-canvas` (grid, axes, trace polylines, dots, regression halos) and a foreground `#overlay-canvas` (crosshair, hover highlights, drag-to-zoom selection box). Hovering a point only redraws `#overlay-canvas` via `requestAnimationFrame` without re-rendering the traces.
- [`PlotSummaryV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/plot-summary-v2-sk.ts#L18-L465) performs synchronous min-max bucket decimation ([`decimate()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/plot-summary-v2-sk.ts#L155-L184)) onto a `<canvas>` and computes selection overlay percentages directly inside `render()` ([`L447-L455`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/plot-summary-v2-sk.ts#L447-L455)) from `.viewportMinX` and `.viewportMaxX`. Because there is no async `google-chart-ready` lifecycle, the summary selection box **cannot desync** from the main charts.

---

## 5. Head-to-Head Architectural Comparison Matrix

| Dimension                          | Legacy (`explore-multi-sk` + `explore-simple-sk` + `dataframe-repository-sk`)                                                                                                                                                                                                                                                 | Multigraph V2 (`explore-multi-v2-sk` + Sub-components)                                                                                                                                                                                                                                                                                   |
| :--------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Component Framework**            | Hybrid of legacy `ElementSk` (`ExploreMultiSk`, `ExploreSimpleSk`, `TestPickerSk`) and `LitElement` (`DataFrameRepository`, `PlotGoogleChartSk`, `PlotSummarySk`).                                                                                                                                                            | 100% modern `LitElement` with `@property` / `@state` unidirectional dataflow.                                                                                                                                                                                                                                                            |
| **Where Canonical State Lives**    | Distributed across **6 stateful layers** (`ExploreMultiSk._state` + 5 parallel arrays, `TestPickerStateController`, $N \times$ `ExploreSimpleSk._state`, $N \times$ `DataFrameRepository`, $N \times$ `PlotGoogleChartSk`, $N \times$ `PlotSummarySk`).                                                                       | **1 place:** [`ExploreMultiV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L78-L135). All child components are controlled views receiving props and emitting semantic events.                                                                                       |
| **Copies of Trace Data in Memory** | **3 copies per chart** (`ExploreMultiSk.allFrameResponses[i]`, `DataFrameRepository._traceset` + `DataTable`, and `ExploreSimpleSk._dataframe` sliced copy) + hidden `exploreElements[0]` accumulator.                                                                                                                        | **1 canonical array:** `ExploreMultiV2Sk._seriesData: TraceSeries[]` + IndexedDB cache ([`TraceDatabase`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/db.ts#L50-L307)).                                                                                                                       |
| **Split-By Implementation**        | Uses hidden `exploreElements[0]` to fetch all data, polls `explore0.spinning` via `rAF`/`setTimeout`, then rips `DataFrame` apart in [`splitGraphs()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-sk/explore-multi-sk.ts#L1308-L1389) and creates/destroys `ExploreSimpleSk` DOM nodes. | Pure function [`computeSplitGroups()`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/chart-logic.ts#L90-L157) called inside `render()`. Zero network calls, zero DOM state tearing, instant toggle.                                                                                             |
| **In-Flight Request Concurrency**  | `ExploreSimpleSk._requestId = 'foo'` mutex rejects concurrent requests with `409 Conflict`; `ExploreMultiSk` uses `setTimeout(..., 0)` and `rAF` spin-locks.                                                                                                                                                                  | Monotonic `_latestRequestId` counter drops stale responses deterministically; `_prefetchAbortController` cancels obsolete background fetches.                                                                                                                                                                                            |
| **Param Filtering & Counts**       | Sequential `/_/nextParamList/` HTTP POST round-trips per dropdown change (`TestPickerStateController`).                                                                                                                                                                                                                       | Off-main-thread Web Worker + `filter.wasm` ([`ExploreWorkerController`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-worker-controller.ts)) over binary inverted index (`traces.bin`).                                                                                                 |
| **Chart Rendering Engine**         | Heavy async `google.visualization.CoreChartBase` + `DataTable` / `DataView` per chart.                                                                                                                                                                                                                                        | Synchronous dual-layer HTML5 `<canvas>` ([`TraceChartSk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/trace-chart-sk.ts)) + min-max decimated `<canvas>` ([`PlotSummaryV2Sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/plot-summary-v2-sk.ts)). |
| **Cross-Chart Hover / Pinning**    | Each `ExploreSimpleSk` manages its own tooltip/selection; `selection-changing-in-multi` events sync X-range only.                                                                                                                                                                                                             | Shared `_globalHoverX` and `_globalPinnedX` props passed to all `TraceChartSk` instances; synchronized crosshair at 60fps on `#overlay-canvas`.                                                                                                                                                                                          |
| **Date vs. Commit Domain Toggle**  | Rebuilds `DataView` columns and triggers `toggleSelectionRange()` hack with known frame-shifting bug (`b/362831653`).                                                                                                                                                                                                         | Toggles `dateMode: boolean` prop; `TraceChartSk` switches X-accessor (`p.x` vs `p.timestamp`) and converts `viewportMinX`/`MaxX` cleanly in [`_toggleDateMode`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1096-L1150).                                              |
| **Client-Side Caching**            | In-memory `allFrameResponses[]` only (lost on reload, bypassed on range extension).                                                                                                                                                                                                                                           | Persistent IndexedDB [`TraceDatabase`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/db.ts) keyed by SHA-256 `hashRequest(req)` for frames, trace values, and commit metadata.                                                                                                                  |

---

## 6. Demo Playbook: 4 Sharp Edges Remaining in V2

If you want to advance from L10 to L11 tomorrow, don't just use [`explore-multi-v2-sk`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts) blindly—know the **four remaining sharp edges** in V2 so you can either steer around them live or land surgical fixes before tomorrow morning:

> [!WARNING]
> **Watch Out for These 4 Specific Edge Cases in `explore-multi-v2-sk` During Tomorrow's Demo:**

### Edge Case 1: Copying the URL Immediately After Editing Queries (Async Shortcut Lag)

- **Where:** [`explore-multi-v2-sk.ts#L345-L374`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L345-L374) and [`_updateShortcut()` (`L1201-L1248`)](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1201-L1248).
- **What happens:** The `stateReflector` getter in `ExploreMultiV2Sk` persists `shortcut: this._shortcut` to the URL query string, and **reads** `qs` / `q` on page load ([`L404-L423`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L404-L423)), but does **not** write `qs` to the URL state object. Instead, when queries change, [`_onQueriesChanged`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1077-L1079) calls `this._updateShortcut()` asynchronously (`POST /_/keys/`) and only updates `this._shortcut` (and the URL) after the RPC returns.
- **Demo Risk:** If `/_/keys/` fails in a local/staging demo environment or user copies the browser URL within ~150ms of adding a query pill, the URL will still point to the _previous_ shortcut.
- **5-Line Fix / Mitigation:** Include `qs` in the `stateReflector` state getter (`() => ({ ..., qs: this.queries.map(...) })`) as an immediate synchronous fallback before `/_/keys/` resolves.

### Edge Case 2: Facet Option Counts Only Computed for `queries[0]`

- **Where:** [`explore-multi-v2-sk.ts#L614-L624`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L614-L624) and [`_triggerWorkerFilter()` (`L656-L664`)](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L656-L664).
- **What happens:** When building `queriesToRun` for leave-one-out facet option counts (`_optionsByKey`), `_triggerWorkerFilter` hardcodes `this.queries[0]`:
  ```ts
  if (this.queries.length > 0) {
    for (const key of Object.keys(this.queries[0])) {
      const qClone = { ...this.queries[0] };
      delete qClone[key];
      queriesToRun[`key_${key}`] = qClone;
    }
  }
  ```
- **Demo Risk:** If you click "+ Add Query to Compare" (`queries[1]`) during the demo, the dropdown option counts in the second query bar will reflect `queries[0]`'s facet constraints rather than `queries[1]`'s.
- **Demo Advice:** Showcase single-query multi-value comparison + Split-By (`splitKeys`) and Diff Base (`_diffBase`), which is V2's strongest workflow, or let me patch `_optionsByKey` to be indexed per query row.

### Edge Case 3: Local Y-Axis Zoom (`_viewportMinY` / `_viewportMaxY`) Sticky Across Summary X-Panning

- **Where:** [`trace-chart-sk.ts#L264-L266`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/trace-chart-sk.ts#L264-L266) and [`updated()` (`L458-L464`)](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/trace-chart-sk.ts#L458-L464).
- **What happens:** `TraceChartSk` stores Y-axis zoom (`_viewportMinY`, `_viewportMaxY`) locally and only resets it when `this.viewportMinX === null` (full Reset Zoom):
  ```ts
  if (this.viewportMinX === null) {
    this._viewportMinY = null;
    this._viewportMaxY = null;
  }
  ```
- **Demo Risk:** If you perform a 2D box zoom on a chart (which sets `_viewportMinY`/`_viewportMaxY` locally) and then drag the `PlotSummaryV2Sk` slider to a different time window where trace Y-values are much higher or lower, `viewportMinX` is non-null so `_viewportMinY`/`_viewportMaxY` stay locked, making the trace appear off-screen vertically until you click Reset Zoom (`0` or double-click).

### Edge Case 4: Formula Pipelines Bypass IndexedDB Cache (`b/381280000`)

- **Where:** [`explore-multi-v2-sk.ts#L1423-L1424`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts#L1423-L1424).
- **What happens:** When `activeFormulas.length > 0` (e.g., `norm()`, `ave()`, `iqrr()`), `cachedResponse` lookup in IndexedDB is intentionally skipped (`TODO(b/381280000): Re-enable client caching once traceId collision is resolved`).
- **Demo Advice:** Pre-warm standard trace queries in IndexedDB before the demo (they will load instantaneously from `TraceDatabase`), and prefer the toolbar's client-side **Normalize (Centre / Scale)**, **Diff Base**, and **Custom Transform (`_transformPreset`)** controls during the live demo—those execute synchronously in [`chart-logic.ts`](file:///usr/local/google/home/mordeckimarcin/buildbot/perf/modules/explore-multi-v2-sk/chart-logic.ts) at 60fps without hitting `/_/frame/start`!
