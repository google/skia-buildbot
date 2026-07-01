import { LitElement, css, html, PropertyValues } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { DataService, TraceValuesRequest, TraceValuesResponse } from '../data-service';
import { FrameRequest, Regression } from '../json';
import { TraceSeries } from './trace-types';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import {
  computeTraceDiffs,
  computeCustomTransforms,
  computeSplitGroups,
  calculateLoadedBounds,
  calculateSharedBounds,
} from './chart-logic';
import { calculateFetchRequests } from './fetch-logic';
import { toParamSet, fromParamSet } from '../../../infra-sk/modules/query';
import { UNSET_TIME } from '../const/const';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { makeKey } from '../paramtools';
import './query-bar-sk';
import { Suggestion } from './query-bar-sk';
import './trace-chart-sk';
import './plot-summary-v2-sk';
import { telemetry } from '../telemetry/telemetry';
import { CountMetric } from '../telemetry/types';

import { TraceDatabase, hashRequest } from './db';
import './explore-toolbar-sk';
import { ExploreWorkerController } from './explore-worker-controller';
import { errorMessage } from '../errorMessage';
import './help-hub-sk';
import './interactive-tour-sk';
import { TourStep } from './interactive-tour-sk';

export const SUBREPO_CONFIG: Record<string, { logUrl: string; repoUrl: string }> = {
  V8: {
    logUrl: 'https://chromium.googlesource.com/v8/v8.git/+log/',
    repoUrl: 'https://chromium.googlesource.com/v8/v8.git/+/',
  },
  WebRTC: {
    logUrl: 'https://webrtc.googlesource.com/src.git/+log/',
    repoUrl: 'https://webrtc.googlesource.com/src.git/+/',
  },
  Skia: {
    logUrl: 'https://skia.googlesource.com/skia.git/+log/',
    repoUrl: 'https://skia.googlesource.com/skia.git/+/',
  },
  Dawn: {
    logUrl: 'https://dawn.googlesource.com/dawn.git/+log/',
    repoUrl: 'https://dawn.googlesource.com/dawn.git/+/',
  },
  Angle: {
    logUrl: 'https://chromium.googlesource.com/angle/angle.git/+log/',
    repoUrl: 'https://chromium.googlesource.com/angle/angle.git/+/',
  },
};

const DEFAULT_SUMMARY_RANGE_SEC = 90 * 24 * 3600; // 90 days
const SUMMARY_INCREMENT_SEC = 90 * 24 * 3600; // 90 days
const ANOMALY_VIEWPORT_PADDING_COMMITS = 100;

@customElement('explore-multi-v2-sk')
export class ExploreMultiV2Sk extends LitElement {
  private _queries: Record<string, string[]>[] = [{}];

  @property({ attribute: false })
  get queries(): Record<string, string[]>[] {
    return this._queries;
  }

  set queries(val: Record<string, string[]>[]) {
    const oldVal = this._queries;
    this._queries = val;
    this._tracePage = 0;
    this._summaryBeginOffsetSec = DEFAULT_SUMMARY_RANGE_SEC;
    this._summaryEndOffsetSec = 0;
    this.requestUpdate('queries', oldVal);
  }

  @property({ type: Boolean }) embedded = false;

  @property({ type: Array }) highlightAnomalies: string[] = [];

  @state() private _queriesExpanded = false;

  @state() private _shortcut = '';

  private _lastQueriesJson = '';

  private _summaryBeginOffsetSec = DEFAULT_SUMMARY_RANGE_SEC;

  private _summaryEndOffsetSec = 0;

  private _lastLoadedShortcut = '';

  @state() private _tourActive = false;

  private _tourSteps: TourStep[] = [
    {
      selector: '.workspace',
      title: 'Dynamic Setup',
      text: "Let's start by loading some comparison data.",
      placement: 'bottom',
    },
    {
      selector: 'query-bar-sk',
      title: 'Faceted Search Bar',
      text: 'Type here to search. It supports fuzzy matching.',
      placement: 'bottom',
    },
    {
      selector: '.suggestions-dropdown',
      title: 'Suggestions List',
      text: 'Here you can see the auto-complete suggestions.',
      placement: 'bottom',
    },
    {
      selector: '.multiselect-trigger',
      title: 'Multi-select Chips',
      text: 'Click a chip to see multi-select options.',
      placement: 'bottom',
    },
    {
      selector: '.multiselect-dropdown',
      title: 'Multi-select Dropdown',
      text: 'Here you can select multiple values.',
      placement: 'bottom',
    },
    {
      selector: '.add-query-circle-btn',
      title: 'Multiple Query Rows',
      text: 'Add more rows to compare completely different queries.',
      placement: 'bottom',
    },
    {
      selector: '.config-pill',
      title: 'Toolbar - Split Chart',
      text: 'Split lines into individual graphs.',
      placement: 'bottom',
    },
    {
      selector: '.date-mode-checkbox',
      title: 'Toolbar - Date Mode',
      text: 'Toggle between commit and date axis.',
      placement: 'bottom',
    },
    {
      selector: '.smooth-checkbox',
      title: 'Toolbar - Smoothing',
      text: 'Adjust curve smoothing.',
      placement: 'bottom',
    },
    {
      selector: '.subrepo-select',
      title: 'Toolbar - Subrepos',
      text: 'Filter traces by sub-repository.',
      placement: 'bottom',
    },
    {
      selector: 'trace-chart-sk',
      title: 'Chart Tooltip',
      text: 'Hover over points to see specific values.',
      placement: 'top',
    },
  ];

  @state() private _defaultParamSelections: Record<string, string[]> = {};

  @state() private _conditionalDefaults: any[] = [];

  @state() private _defaults: any = null;

  @state() private _workerInitializing = true;

  @state() private _loading = false;

  @state() private _statusMessage = '';

  @state() private _availableParams: { key: string; value: string; count: number }[] = [];

  @state() private _optionsByKey: Record<string, { value: string; count: number }[]> = {};

  @state() private _availableParamsPerQuery: { key: string; value: string; count: number }[][] = [];

  @state() private _optionsByKeyPerQuery: Record<string, { value: string; count: number }[]>[] = [];

  @state() private _suggestionsForQueryBar: Suggestion[][] = [];

  private _latestSuggestRequestIds: number[] = [];

  @property({ attribute: false }) splitKeys: Set<string> = new Set();

  @state() private _includeParams: string[] = [];

  @state() private _normalizeCentre: 'none' | 'first' | 'average' | 'median' = 'none';

  @state() private _normalizeScale: 'none' | 'minmax' | 'stddev' = 'none';

  @state() private _hoverMode: 'original' | 'smoothed' | 'both' = 'original';

  @state() private _smoothingRadius = 20;

  @property({ type: Boolean }) dateMode = false;

  @state() private _edgeDetectionFactor = 1.0;

  @state() private _edgeLookahead = 3;

  @state() private _showDots = true;

  @state() private _seriesData: TraceSeries[] = [];

  @property({ type: Number, reflect: true }) viewportMinX: number | null = null;

  @property({ type: Number, reflect: true }) viewportMaxX: number | null = null;

  private _initialViewportBounds: { minX: number; maxX: number } | null = null;

  @state() private _globalHoverX: number | null = null;

  @state() private _globalPinnedX: number | null = null;

  @state() private _loadedBounds: Record<string, { min: number; max: number }> = {};

  @state() private _globalBounds: Record<string, { min: number; max: number }> = {};

  @state() private _selectedSubrepo: string = 'none';

  @state() private _availableSubrepos: string[] = [];

  @state() private _diffBase: { key: string; value: string } | null = null;

  @state() private _pageSize = 10;

  @state() private _showRegressions = true;

  @state() private _showSparklines = false;

  @state() private _summaryLoading = false;

  private _prefetchAbortController: AbortController | null = null;

  @state() private _evenXAxisSpacing = false;

  @state() private _transformPreset = 'none';

  @state() private _tooltipDiffs = false;

  @state() private _smooth = false;

  @state() private _regressions: { [trace_id: string]: { [commit: number]: Regression } } = {};

  @state() private _tracePage = 0;

  @state() private _showSummaryBar = true;

  @state() private _showAllTraces = false;

  @property({ type: Number }) begin = UNSET_TIME;

  @property({ type: Number }) end = UNSET_TIME;

  @state() private _user = '';

  private _workerController: ExploreWorkerController | null = null;

  private _latestActiveFacets: string[] = [];

  private _viewportChangeTimeout: any = null;

  @state() private _matchingTraceIds: string[] = [];

  private _stateHasChanged: () => void = () => {};

  private _inFlightMetadataCommits = new Set<number>();

  private _latestRequestId = 0;

  private _debounceDelay = 100;

  private _workerFilterTimer: number | null = null;

  private _fetchDataTimer: number | null = null;

  private _fetchMetadataTimer: number | null = null;

  connectedCallback() {
    super.connectedCallback();

    telemetry.increaseCounter(CountMetric.ExploreMultiV2Visit);

    LoggedIn()
      .then((status: LoginStatus) => {
        this._user = status.email;
      })
      .catch((e) => console.error('Failed to check login status', e));

    const db = new TraceDatabase();
    db.evictOlderThan(30).catch((e: any) => console.error('Eviction failed:', e));

    let isInitialGetState = true;
    this._stateHasChanged = stateReflector(
      () => {
        const stateObj: Record<string, any> = {
          shortcut: this._shortcut,
          centre: this._normalizeCentre,
          scale: this._normalizeScale,
          hoverMode: this._hoverMode,
          radius: this._smoothingRadius,
          dots: this._showDots,
          split: Array.from(this.splitKeys).join(','),
          diff_base: this._diffBase ? `${this._diffBase.key}=${this._diffBase.value}` : '',
          sparklines: this._showSparklines,

          regressions: this._showRegressions,
          tooltipDiffs: this._tooltipDiffs,
          evenXAxisSpacing: this._evenXAxisSpacing,
          transformPreset: this._transformPreset,
          dateMode: this.dateMode,
          page: this._tracePage,
          pageSize: this._pageSize,
          showAll: this._showAllTraces,
          subrepo: this._selectedSubrepo,
          edgeFactor: this._edgeDetectionFactor,
          outlier: this._edgeLookahead,
          begin: this.begin,
          end: this.end,
        };

        // Dynamically preserve all non-managed query parameters from the URL (e.g., from report-page or other elements)
        const urlParams = new URLSearchParams(window.location.search);
        const managedKeys = new Set(Object.keys(stateObj));
        urlParams.forEach((value, key) => {
          if (!managedKeys.has(key)) {
            stateObj[key] = value;
          }
        });

        if (isInitialGetState) {
          isInitialGetState = false;
          // Exclude non-managed keys on the very first capture so stateReflector registers them as dirty/always-serialize
          urlParams.forEach((_, key) => {
            if (!managedKeys.has(key)) {
              delete stateObj[key];
            }
          });
        }

        return stateObj;
      },
      (o: any) => {
        const stateObj = o as any;
        if (stateObj.shortcut) {
          this._shortcut = stateObj.shortcut;
          void this._loadShortcut(this._shortcut);
        } else {
          this._shortcut = stateObj.shortcut || '';
          if (stateObj.qs !== undefined) {
            console.log('explore-multi-v2-sk: Raw qs from URL:', stateObj.qs);
            try {
              const parsed = JSON.parse(stateObj.qs);
              if (Array.isArray(parsed)) {
                this.queries = parsed;
              } else if (typeof parsed === 'object' && parsed !== null) {
                console.log('explore-multi-v2-sk: Wrapping object query in array');
                this.queries = [parsed];
              } else {
                console.log('explore-multi-v2-sk: Invalid qs type:', typeof parsed);
                this.queries = [{}];
              }
            } catch (e) {
              console.error('explore-multi-v2-sk: Failed to parse qs from URL:', e);
              this.queries = [{}];
            }
          } else if (stateObj.q !== undefined) {
            this.queries = [toParamSet(stateObj.q)];
          }
        }
        if (stateObj.centre !== undefined) this._normalizeCentre = stateObj.centre;
        if (stateObj.scale !== undefined) this._normalizeScale = stateObj.scale;
        if (stateObj.hoverMode !== undefined) {
          this._hoverMode = stateObj.hoverMode;
          this._smooth = this._hoverMode === 'both' || this._hoverMode === 'smoothed';
        }
        if (stateObj.radius !== undefined) this._smoothingRadius = stateObj.radius;
        if (stateObj.dots !== undefined) this._showDots = stateObj.dots;
        if (stateObj.split !== undefined) {
          this.splitKeys = new Set(stateObj.split ? stateObj.split.split(',') : []);
        }
        if (stateObj.diff_base) {
          const parts = stateObj.diff_base.split('=');
          if (parts.length === 2) {
            this._diffBase = { key: parts[0], value: parts[1] };
          }
        } else {
          this._diffBase = null;
        }
        if (stateObj.sparklines !== undefined) this._showSparklines = stateObj.sparklines;

        if (stateObj.regressions !== undefined) this._showRegressions = stateObj.regressions;
        if (stateObj.tooltipDiffs !== undefined) this._tooltipDiffs = stateObj.tooltipDiffs;
        if (stateObj.evenXAxisSpacing !== undefined)
          this._evenXAxisSpacing = stateObj.evenXAxisSpacing;
        if (stateObj.transformPreset !== undefined)
          this._transformPreset = stateObj.transformPreset;
        if (stateObj.dateMode !== undefined) this.dateMode = stateObj.dateMode;
        if (stateObj.page !== undefined) this._tracePage = stateObj.page;
        if (stateObj.pageSize !== undefined) this._pageSize = stateObj.pageSize;
        if (stateObj.showAll !== undefined) this._showAllTraces = stateObj.showAll;
        if (stateObj.subrepo !== undefined) this._selectedSubrepo = stateObj.subrepo;
        if (stateObj.edgeFactor !== undefined) this._edgeDetectionFactor = stateObj.edgeFactor;
        if (stateObj.outlier !== undefined) this._edgeLookahead = stateObj.outlier;
        if (stateObj.begin !== undefined) this.begin = Number(stateObj.begin);
        if (stateObj.end !== undefined) this.end = Number(stateObj.end);
      },
      true
    );

    this._initWorker();
    window.addEventListener('anomalies-source-changed', this._onAnomaliesSourceChanged);
    this.addEventListener('anomaly-changed', this._onAnomalyChanged);
  }

  disconnectedCallback() {
    if (this._prefetchAbortController) {
      this._prefetchAbortController.abort();
    }
    if (this._workerController) {
      this._workerController.terminate();
    }
    if (this._workerFilterTimer !== null) {
      window.clearTimeout(this._workerFilterTimer);
    }
    if (this._fetchDataTimer !== null) {
      window.clearTimeout(this._fetchDataTimer);
    }
    if (this._fetchMetadataTimer !== null) {
      window.clearTimeout(this._fetchMetadataTimer);
    }
    window.removeEventListener('anomalies-source-changed', this._onAnomaliesSourceChanged);
    this.removeEventListener('anomaly-changed', this._onAnomalyChanged);
    super.disconnectedCallback();
  }

  private _onAnomaliesSourceChanged = () => {
    this._seriesData = [];
    this._loadedBounds = {};
    this._globalBounds = {};
    this._regressions = {};
    this._latestRequestId++;
    void this._fetchData(this._latestRequestId);
  };

  private _initWorker() {
    this._workerInitializing = true;
    this._loading = true;
    this._statusMessage = 'Starting worker...';
    this._workerController = new ExploreWorkerController(
      () => {
        console.log('Orchestrator: Worker loaded');
      },
      () => {
        console.log('Orchestrator: Worker ready');
        this._workerInitializing = false;
        this._latestRequestId = this._triggerWorkerFilter();
      },
      (payload) => {
        console.log(`Worker Progress [${payload.name}]: ${payload.loaded}/${payload.total}`);
      },
      (message) => {
        console.error('Worker Error:', message);
        this._loading = false;
        this._statusMessage = '';
        this._workerInitializing = false;
        errorMessage(`Worker initialization failed: ${message}`);
      },
      (payload) => {
        console.log('Worker Params Ready');
        if (payload.availableParams) {
          this._availableParams = payload.availableParams;
        }
        if (payload.paramsByKey) {
          this._optionsByKey = payload.paramsByKey;
        }
      },
      (payload) => {
        this._handleFilterResult(payload);
      },
      (payload, idx, requestId) => {
        if (requestId !== this._latestSuggestRequestIds[idx]) {
          return;
        }
        const newSuggestions = [...this._suggestionsForQueryBar];
        newSuggestions[idx] = payload;
        this._suggestionsForQueryBar = newSuggestions;
      },
      (message) => {
        this._statusMessage = message;
      }
    );
    this._workerController.init();
  }

  private _handleFilterResult(payload: any) {
    console.log('Worker Filter Result:', payload.filteredCount);

    if (payload.requestId !== undefined && payload.requestId !== this._latestRequestId) {
      console.log(
        `Discarding stale worker result: expected ${this._latestRequestId}, got ${payload.requestId}`
      );
      return;
    }

    const reconstructedIds: string[] = [];
    if (payload.results) {
      payload.results.forEach((r: any) => {
        const paramsObj: Record<string, string> = {};
        r.params.forEach((p: any) => {
          paramsObj[p.key] = p.value;
        });
        try {
          const key = makeKey(paramsObj);
          reconstructedIds.push(key);
        } catch (err) {
          console.error('Failed to construct key for trace:', r.index, err);
        }
      });
    }
    const queryObj = this.queries[0];
    const hasFilters = queryObj
      ? Object.values(queryObj).some((arr) => arr && arr.length > 0)
      : false;

    if (!hasFilters) {
      this._matchingTraceIds = [];
      this._updateSeriesData([]);
      this._loadedBounds = {};
      this._globalBounds = {};
      this._regressions = {};
      this._loading = false;
      this._statusMessage = '';
    } else {
      this._matchingTraceIds = reconstructedIds;
      console.log(`Reconstructed ${reconstructedIds.length} matching Trace IDs`);
      void this._fetchData(this._latestRequestId);
    }

    if (payload.queryResults && payload.queryResults[0]) {
      const firstResult = payload.queryResults[0];
      if (firstResult.availableParams) {
        this._availableParams = firstResult.availableParams;
      }

      const mergedOptionsByKey = firstResult.paramsByKey ? { ...firstResult.paramsByKey } : {};

      // Override counts for active facets with data from corresponding results
      if (payload.queryResults.length > 1 && this._latestActiveFacets.length > 0) {
        this._latestActiveFacets.forEach((key, index) => {
          const resultIdx = index + this.queries.length;
          if (payload.queryResults[resultIdx]) {
            const facetResult = payload.queryResults[resultIdx];
            if (facetResult.paramsByKey && facetResult.paramsByKey[key]) {
              mergedOptionsByKey[key] = facetResult.paramsByKey[key];
            }
          }
        });
      }

      this._optionsByKey = mergedOptionsByKey;

      // Store independent params and options for each query bar
      const availableParamsPerQuery: { key: string; value: string; count: number }[][] = [];
      const optionsByKeyPerQuery: Record<string, { value: string; count: number }[]>[] = [];

      payload.queryResults.forEach((result: any, idx: number) => {
        if (idx < this.queries.length) {
          availableParamsPerQuery.push(result.availableParams || []);
          if (idx === 0) {
            optionsByKeyPerQuery.push(mergedOptionsByKey);
          } else {
            optionsByKeyPerQuery.push(result.paramsByKey || {});
          }
        }
      });

      this._availableParamsPerQuery = availableParamsPerQuery;
      this._optionsByKeyPerQuery = optionsByKeyPerQuery;
    }
  }

  private _triggerWorkerFilter(requestId?: number): number {
    if (!this._workerController?.isReady()) return -1;

    this._statusMessage = ''; // Clear stale status messages from initialization.
    const queries: Record<string, string[]>[] = [...this.queries];
    const activeFacets: string[] = [];

    // Add one query per selected facet, with that facet removed
    for (const key of Object.keys(this.queries[0])) {
      if (this.queries[0][key] && this.queries[0][key].length > 0) {
        activeFacets.push(key);
        const queryCopy = { ...this.queries[0] };
        delete queryCopy[key];
        queries.push(queryCopy);
      }
    }

    this._latestActiveFacets = activeFacets;

    return this._workerController.filter(queries, this.queries.length, requestId);
  }

  static styles = css`
    :host {
      display: block;
      padding: 16px;
      font-family: var(--font, 'Inter', system-ui, -apple-system, sans-serif);
      background-color: var(--background);
      color: var(--on-background);
      min-height: 100vh;
    }

    .range-menu {
      background-color: var(--surface);
      border: 1px solid var(--border);
      border-radius: 4px;
      box-shadow:
        0 4px 6px -1px color-mix(in srgb, var(--transparent-overlay) 20%, transparent),
        0 2px 4px -1px color-mix(in srgb, var(--transparent-overlay) 12%, transparent);
      padding: 8px;
      display: flex;
      flex-direction: column;
      gap: 4px;
      z-index: 1000;
    }

    .range-menu button {
      background: none;
      border: none;
      color: var(--on-surface);
      text-align: left;
      padding: 4px 8px;
      cursor: pointer;
      border-radius: 2px;
    }

    .page-loading {
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      gap: 12px;
      color: var(--on-surface);
      font-size: 16px;
      padding: 40px;
    }

    .spinner {
      width: 32px;
      height: 32px;
      border: 3px solid color-mix(in srgb, var(--on-surface) 10%, transparent);
      border-radius: 50%;
      border-top-color: var(--primary);
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }

    .range-menu button:hover {
      background-color: var(--surface-hover);
    }

    .header {
      margin-bottom: 16px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    h1 {
      color: var(--primary);
      font-size: 24px;
      font-weight: 800;
      margin: 0;
      letter-spacing: -0.025em;
    }

    .subtitle {
      color: var(--on-surface);
      font-size: 12px;
      margin: 4px 0 0;
    }

    .workspace {
      background: var(--surface);
      backdrop-filter: blur(12px);
      border: none;
      color: var(--on-surface);
      border-radius: 12px;
      padding: 12px;
      box-shadow:
        0 10px 25px -5px color-mix(in srgb, var(--transparent-overlay) 20%, transparent),
        0 8px 10px -6px color-mix(in srgb, var(--transparent-overlay) 20%, transparent);
    }

    .section-title {
      font-size: 10px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.1em;
      color: var(--on-surface);
      margin-bottom: 8px;
    }

    .charts-container {
      display: flex;
      flex-direction: column;
      gap: 12px;
      margin-top: 12px;
    }

    .sparklines-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
      gap: 12px;
      margin-top: 12px;
    }

    .query-row {
      margin-bottom: 6px;
    }

    .add-query-circle-btn {
      width: 24px;
      height: 24px;
      border-radius: 50%;
      background: var(--surface);
      border: none;
      color: var(--primary);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 16px;
      font-weight: bold;
      cursor: pointer;
      box-shadow: 0 2px 4px color-mix(in srgb, var(--transparent-overlay) 20%, transparent);
      transition: all 0.2s;
    }

    .add-query-circle-btn:hover {
      background: var(--surface-variant);
      box-shadow: 0 4px 6px color-mix(in srgb, var(--transparent-overlay) 30%, transparent);
    }

    .config-pills {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      margin: 12px 0;
    }

    .config-pill {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 4px 12px;
      background: color-mix(in srgb, var(--on-surface) 5%, transparent);
      border-radius: 16px;
      font-size: 12px;
      color: var(--on-surface);
      border: 1px solid var(--outline);
    }

    .config-pill.diff-base {
      background: color-mix(in srgb, var(--primary) 15%, transparent);
      border: 1px solid color-mix(in srgb, var(--primary) 30%, transparent);
    }

    .config-pill-label {
      font-weight: 600;
      color: var(--primary);
    }

    .config-pill-value {
      max-width: 300px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .config-pill-remove {
      border: none;
      background: none;
      cursor: pointer;
      font-size: 14px;
      color: var(--on-surface);
      padding: 0;
      margin-left: 4px;
      line-height: 1;
      display: flex;
      align-items: center;
    }

    .config-pill-remove:hover {
      color: var(--on-background);
    }

    .expand-queries-btn {
      background: var(--surface);
      border: 1px solid var(--outline);
      color: var(--on-surface);
      border-radius: 16px;
      padding: 4px 12px;
      font-size: 11px;
      font-weight: 600;
      cursor: pointer;
      box-shadow: 0 2px 4px color-mix(in srgb, var(--transparent-overlay) 20%, transparent);
      transition: all 0.2s;
      display: inline-flex;
      align-items: center;
      justify-content: center;
    }

    .expand-queries-btn:hover {
      background: var(--surface-hover);
      border-color: var(--primary);
      color: var(--on-background);
      box-shadow: 0 4px 6px color-mix(in srgb, var(--transparent-overlay) 30%, transparent);
    }

    .worker-init-overlay {
      position: fixed;
      bottom: 24px;
      right: 24px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 8px;
      box-shadow: 0 4px 12px color-mix(in srgb, var(--transparent-overlay) 60%, transparent);
      padding: 12px 20px;
      display: flex;
      align-items: center;
      gap: 12px;
      z-index: 2000;
    }

    .worker-init-status {
      font-size: 14px;
      font-weight: 500;
      color: var(--on-background);
    }

    .worker-init-overlay .spinner {
      border: 2px solid var(--border);
      width: 20px;
      height: 20px;
      border-radius: 50%;
      border-left-color: var(--primary);
      animation: spin 1s linear infinite;
    }
  `;

  willUpdate(changedProperties: PropertyValues) {
    super.willUpdate(changedProperties);
    if (changedProperties.has('dateMode')) {
      this._initialViewportBounds = null;
    }
    if (
      this._initialViewportBounds === null &&
      this.viewportMinX !== null &&
      this.viewportMaxX !== null
    ) {
      this._initialViewportBounds = {
        minX: this.viewportMinX,
        maxX: this.viewportMaxX,
      };
    }
  }

  protected firstUpdated() {
    // Yield a macro-tick to guarantee stateReflector's initial stateFromURL microtask
    // completes first and sets loaded=true. This ensures resolved default/partial bounds
    // are successfully written back to the URL rather than being ignored and overwritten.
    setTimeout(() => {
      void this._fetchMetadata();
    }, 0);
  }

  private async _fetchMetadata() {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    try {
      const json = await DataService.getInstance().getInitPage(tz);
      const defaults = await DataService.getInstance().getDefaults();
      this._defaults = defaults;
      this._includeParams = defaults.include_params || [];
      this._defaultParamSelections =
        (defaults.default_param_selections as Record<string, string[]>) || {};
      this._conditionalDefaults = defaults.conditional_defaults || [];
      this._applyConfigDefaults(defaults);

      // Apply defaults to initial query if empty and no shortcut is present
      if (
        this.queries.length === 1 &&
        Object.keys(this.queries[0]).length === 0 &&
        !this._shortcut
      ) {
        this.queries = [{ ...this._defaultParamSelections }];
      }

      if (json && json.dataframe && json.dataframe.paramset) {
        const paramset = json.dataframe.paramset as Record<string, string[]>;
        const optionsByKey: Record<string, { value: string; count: number }[]> = {};
        const availableParams: { key: string; value: string; count: number }[] = [];

        Object.keys(paramset).forEach((key) => {
          optionsByKey[key] = paramset[key].map((v) => ({ value: v, count: 0 }));
          paramset[key].forEach((v) => {
            availableParams.push({ key: key, value: v, count: 0 });
          });
        });

        this._optionsByKey = optionsByKey;
        this._availableParams = availableParams;
      }
    } catch (e) {
      console.error('Metadata fetch error:', e);
    } finally {
      this._resolveTimeRange();
    }
  }

  private _applyConfigDefaults(defaults: any) {
    if (!defaults) return;
    const urlParams = new URLSearchParams(window.location.search);

    if (defaults.default_xaxis_domain === 'date' && !urlParams.has('dateMode')) {
      this.dateMode = true;
    }

    if (defaults.default_url_values) {
      const urlVals = defaults.default_url_values;
      const stringToBool = (val: string) => val.toLowerCase() === 'true';

      if (urlVals.evenXAxisSpacing !== undefined && !urlParams.has('evenXAxisSpacing')) {
        this._evenXAxisSpacing = stringToBool(urlVals.evenXAxisSpacing);
      }
      if (urlVals.plotSummary !== undefined && !urlParams.has('plotSummary')) {
        this._showSummaryBar = stringToBool(urlVals.plotSummary);
      }
      if (urlVals.dots !== undefined && !urlParams.has('dots')) {
        this._showDots = stringToBool(urlVals.dots);
      }
      if (urlVals.sparklines !== undefined && !urlParams.has('sparklines')) {
        this._showSparklines = stringToBool(urlVals.sparklines);
      }
      if (urlVals.regressions !== undefined && !urlParams.has('regressions')) {
        this._showRegressions = stringToBool(urlVals.regressions);
      }
      if (urlVals.tooltipDiffs !== undefined && !urlParams.has('tooltipDiffs')) {
        this._tooltipDiffs = stringToBool(urlVals.tooltipDiffs);
      }
      if (urlVals.transformPreset !== undefined && !urlParams.has('transformPreset')) {
        this._transformPreset = urlVals.transformPreset;
      }
    }
  }

  protected updated(changedProperties: PropertyValues) {
    let queriesChanged = false;
    if (changedProperties.has('queries')) {
      queriesChanged = true;
    }

    const pageOrSizeChanged =
      changedProperties.has('_tracePage') ||
      changedProperties.has('_pageSize') ||
      changedProperties.has('_showAllTraces');

    const needsNewRequest = queriesChanged || (!queriesChanged && pageOrSizeChanged);

    if (needsNewRequest) {
      this._latestRequestId++;
    }

    const currentRequestId = this._latestRequestId;

    if (queriesChanged) {
      if (this._workerController?.isReady()) {
        if (this._debounceDelay === 0) {
          this._triggerWorkerFilter(currentRequestId);
        } else {
          if (this._workerFilterTimer !== null) {
            window.clearTimeout(this._workerFilterTimer);
          }
          this._workerFilterTimer = window.setTimeout(() => {
            this._workerFilterTimer = null;
            this._triggerWorkerFilter(currentRequestId);
          }, this._debounceDelay);
        }
      }
      void this._updateShortcut();
    }

    if (!queriesChanged && pageOrSizeChanged) {
      if (this._debounceDelay === 0) {
        void this._fetchData(currentRequestId);
      } else {
        if (this._fetchDataTimer !== null) {
          window.clearTimeout(this._fetchDataTimer);
        }
        this._fetchDataTimer = window.setTimeout(() => {
          this._fetchDataTimer = null;
          void this._fetchData(currentRequestId);
        }, this._debounceDelay);
      }
    }

    if (
      changedProperties.has('_seriesData') ||
      (!queriesChanged &&
        (changedProperties.has('_tracePage') || changedProperties.has('_pageSize')))
    ) {
      if (this._debounceDelay === 0) {
        void this._fetchMetadataForVisibleTraces(currentRequestId);
      } else {
        if (this._fetchMetadataTimer !== null) {
          window.clearTimeout(this._fetchMetadataTimer);
        }
        this._fetchMetadataTimer = window.setTimeout(() => {
          this._fetchMetadataTimer = null;
          void this._fetchMetadataForVisibleTraces(currentRequestId);
        }, this._debounceDelay);
      }
    }

    if (
      changedProperties.has('queries') ||
      changedProperties.has('_normalizeCentre') ||
      changedProperties.has('_normalizeScale') ||
      changedProperties.has('_hoverMode') ||
      changedProperties.has('_smoothingRadius') ||
      changedProperties.has('_showDots') ||
      changedProperties.has('splitKeys') ||
      changedProperties.has('_diffBase') ||
      changedProperties.has('_showSparklines') ||
      changedProperties.has('dateMode') ||
      changedProperties.has('_tracePage') ||
      changedProperties.has('_pageSize') ||
      changedProperties.has('_showAllTraces') ||
      changedProperties.has('_selectedSubrepo') ||
      changedProperties.has('_edgeDetectionFactor') ||
      changedProperties.has('_edgeLookahead') ||
      changedProperties.has('_showRegressions') ||
      changedProperties.has('_tooltipDiffs') ||
      changedProperties.has('_evenXAxisSpacing') ||
      changedProperties.has('begin') ||
      changedProperties.has('end')
    ) {
      this._stateHasChanged();
    }
  }

  private async _loadShortcut(id: string) {
    if (!id || id === this._lastLoadedShortcut) return;
    this._lastLoadedShortcut = id;
    this._loading = true;
    try {
      const graphConfigs = await DataService.getInstance().getShortcut(id);
      if (graphConfigs && graphConfigs.length > 0) {
        const queries: Record<string, string[]>[] = [];
        for (const config of graphConfigs) {
          if (config.queries && config.queries.length > 0) {
            for (const q of config.queries) {
              queries.push(toParamSet(q));
            }
          }
        }
        if (queries.length > 0) {
          this._lastQueriesJson = JSON.stringify(queries);
          this.queries = queries;
        }
      }
    } catch (e) {
      console.error('Failed to load shortcut:', e);
    } finally {
      this._loading = false;
    }
  }

  private async _updateShortcut() {
    if ((window as any).perf && (window as any).perf.disable_shortcut_update) {
      return;
    }
    const currentJson = JSON.stringify(this.queries);
    if (currentJson === this._lastQueriesJson) {
      return;
    }
    this._lastQueriesJson = currentJson;

    const graphConfigs = this.queries
      .filter((q) => Object.keys(q).length > 0)
      .map((q) => {
        const config = new GraphConfig();
        config.queries = [fromParamSet(q)];
        return config;
      });

    if (graphConfigs.length === 0) {
      if (this._shortcut !== '') {
        this._shortcut = '';
        this._stateHasChanged();
      }
      return;
    }

    try {
      const id = await updateShortcut(graphConfigs);
      if (id && id !== this._shortcut) {
        this._shortcut = id;
        this._stateHasChanged();
      }
    } catch (e) {
      console.error('Failed to update shortcut:', e);
    }
  }

  private _mapAnomalyFields(anomaly: any): Regression {
    return {
      ...anomaly,
      start_revision: anomaly.start_revision,
      end_revision: anomaly.end_revision,
      is_improvement: anomaly.is_improvement,
      bug_id: anomaly.bug_id,
      recovered: anomaly.recovered,
      status: anomaly.state,
      median_before: anomaly.median_before_anomaly,
      median_after: anomaly.median_after_anomaly,
      test_path: anomaly.test_path || (anomaly as any).TestPath,
    } as Regression;
  }

  /**
   * Resolves and calculates the final begin and end Unix timestamps for data fetching.
   *
   * Calculations performed:
   * 1. Both begin & end provided (begin !== UNSET_TIME && end !== UNSET_TIME):
   *    - Uses them exactly as-is.
   * 2. Only begin provided (begin !== UNSET_TIME):
   *    - Calculates `end = begin + defaultRange`. If in future, caps at `now`.
   * 3. Only end provided (end !== UNSET_TIME):
   *    - Calculates `begin = end - defaultRange`.
   * 4. Neither provided (initial load):
   *    - Calculates `end = now` and `begin = now - defaultRange`.
   * 5. If both are equal (begin === end):
   *    - Centers range: `begin = begin - halfRange`, `end = end + halfRange`.
   *    - If `end` extends into the future, shifts the entire range backward so `end = now`.
   *
   * Side Effects:
   * - Instantly writes resolved default/partial timestamps back to `this._begin` and `this._end`
   *   so `stateReflector` serializes them directly to the browser URL on load.
   *
   * @returns Calculated begin and end timestamps.
   */
  private _resolveTimeRange(): { begin: number; end: number } {
    let now = Math.floor(Date.now() / 1000);
    if ((window as any).perf?.demo) {
      // The demo dataset resides on March 22, 2020. Lock now anchor to April 1, 2020
      // so the standard 150-day lookback window correctly encompasses the historical files.
      now = Math.floor(new Date('2020-04-01T00:00:00Z').getTime() / 1000);
    }
    const defaultRangeS = this._defaults?.default_range || 150 * 24 * 3600;

    let begin = this.begin;
    let end = this.end;

    const beginProvided = begin !== UNSET_TIME;
    const endProvided = end !== UNSET_TIME;

    if (beginProvided || endProvided) {
      if (!beginProvided) {
        begin = end - defaultRangeS;
      } else if (!endProvided) {
        end = begin + defaultRangeS;
        if (end > now) end = now;
      } else if (begin === end) {
        const halfRange = Math.floor(defaultRangeS / 2);
        begin = begin - halfRange;
        end = end + halfRange;
        if (end > now) {
          const shift = end - now;
          end = now;
          begin -= shift;
        }
      }
    } else {
      begin = now - defaultRangeS;
      end = now;
    }

    const resolvedBegin = Math.round(begin);
    const resolvedEnd = Math.round(end);

    // Write back defaults/partials to keep the URL deterministic
    if (!beginProvided) {
      this.begin = resolvedBegin;
    }
    if (!endProvided) {
      this.end = resolvedEnd;
    }

    return {
      begin: resolvedBegin,
      end: resolvedEnd,
    };
  }

  private async _fetchData(requestId: number, retryCount = 0): Promise<void> {
    console.log('[_fetchData] called, retryCount:', retryCount, 'requestId:', requestId);
    if (requestId !== this._latestRequestId) {
      console.log('[_fetchData] aborted early: request ID mismatch');
      return;
    }
    const startIdx = this._tracePage * this._pageSize;
    const endIdx = startIdx + this._pageSize;
    const visibleIds = this._showAllTraces
      ? this._matchingTraceIds.slice(0, 500)
      : this._matchingTraceIds.slice(startIdx, endIdx);
    if (visibleIds.length === 0) {
      return;
    }

    const loadedIds = new Set(this._seriesData.map((s) => s.id));

    this._loading = true;
    try {
      const { begin, end } = this._resolveTimeRange();
      const quantizedNow = Math.floor(end / 3600) * 3600;
      let quantizedBegin = Math.floor(begin / 3600) * 3600;
      if (retryCount > 0) {
        const duration = (end - begin) * Math.pow(2, retryCount);
        quantizedBegin = quantizedNow - duration;
      }

      let reqTraceIds = [...visibleIds];

      reqTraceIds = Array.from(new Set(reqTraceIds));
      console.log('[_fetchData] reqTraceIds:', reqTraceIds);
      if (reqTraceIds.every((id) => loadedIds.has(id))) {
        console.log('Skipping fetch: all requested traces already loaded in memory');
        if (requestId === this._latestRequestId) {
          this._loading = false;
        }
        return;
      }

      const req: FrameRequest = {
        begin: quantizedBegin,
        end: quantizedNow,
        trace_ids: reqTraceIds,
        tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      };

      const cacheKey = await hashRequest(req, (window as any).perf?.fetch_anomalies_from_sql);
      const db = new TraceDatabase();
      const cached = await db.get(cacheKey);
      if (requestId !== this._latestRequestId) return;
      if (cached) {
        console.log('Serving from cache:', cacheKey);

        if (cached.anomalymap) {
          const nextRegressions = { ...this._regressions };
          for (const [traceId, commitMap] of Object.entries(cached.anomalymap)) {
            if (!commitMap) continue;

            const primaryKey = traceId;

            if (!nextRegressions[primaryKey]) {
              nextRegressions[primaryKey] = {};
            }
            for (const [commit, anomaly] of Object.entries(commitMap)) {
              nextRegressions[primaryKey][Number(commit)] = this._mapAnomalyFields(anomaly);
            }
          }
          this._regressions = nextRegressions;
        }

        if (cached.dataframe) {
          const newSeries = this._translateDataFrame(cached.dataframe);
          this._processNewSeries(newSeries, true);
          void this._prefetchHistory(requestId);
        }
        return;
      }

      const response = await DataService.getInstance().sendFrameRequest(req, {
        onProgress: (prog: string) => console.log('Progress:', prog),
        onMessage: (msg: string) => console.error('Message:', msg),
      });

      if (requestId !== this._latestRequestId) {
        if (response && response.dataframe) {
          await db.set(cacheKey, response);
        }
        return;
      }

      if (response && response.anomalymap) {
        const nextRegressions = { ...this._regressions };
        for (const [traceId, commitMap] of Object.entries(response.anomalymap)) {
          if (!commitMap) continue;

          const primaryKey = traceId;

          if (!nextRegressions[primaryKey]) {
            nextRegressions[primaryKey] = {};
          }
          for (const [commit, anomaly] of Object.entries(commitMap)) {
            nextRegressions[primaryKey][Number(commit)] = this._mapAnomalyFields(anomaly);
          }
        }
        this._regressions = nextRegressions;
      }

      if (response && response.dataframe) {
        const newSeries = this._translateDataFrame(response.dataframe);

        if (newSeries.length === 0 && retryCount < 6) {
          console.log(
            'Out of bounds empty traceset detected. Widening duration bounds retry:',
            retryCount + 1
          );
          return await this._fetchData(requestId, retryCount + 1);
        }

        // If traceset is empty, render empty chart.
        this._processNewSeries(newSeries, true);
        await db.set(cacheKey, response);
        void this._prefetchHistory(requestId);
      }
    } catch (e) {
      console.error('Fetch error:', e);
    } finally {
      if (requestId === this._latestRequestId) {
        this._loading = false;
      }
    }
  }

  private _getLoadedRowsRange(): { min: number; max: number } | null {
    let min = Infinity;
    let max = -Infinity;
    for (const s of this._seriesData) {
      if (s.rows) {
        for (const r of s.rows) {
          if (r.createdat !== undefined && r.createdat > 0) {
            if (r.createdat < min) min = r.createdat;
            if (r.createdat > max) max = r.createdat;
          }
        }
      }
    }
    if (min === Infinity || max === -Infinity) {
      return null;
    }
    return { min, max };
  }

  private async _prefetchHistory(requestId: number): Promise<void> {
    console.log(
      `[_prefetchHistory] initiating background prefetch from -${this._summaryBeginOffsetSec}s to +${this._summaryEndOffsetSec}s`
    );
    if (this._prefetchAbortController) {
      this._prefetchAbortController.abort();
    }
    this._prefetchAbortController = new AbortController();
    const signal = this._prefetchAbortController.signal;

    const startIdx = this._tracePage * this._pageSize;
    const endIdx = startIdx + this._pageSize;
    const visibleIds = this._showAllTraces
      ? this._matchingTraceIds.slice(0, 500)
      : this._matchingTraceIds.slice(startIdx, endIdx);

    if (visibleIds.length === 0) return;

    const { end } = this._resolveTimeRange();
    let prefetchBegin = end - this._summaryBeginOffsetSec;
    let prefetchEnd = end + this._summaryEndOffsetSec;

    const existingRange = this._getLoadedRowsRange();
    if (existingRange) {
      if (prefetchBegin > existingRange.min) {
        prefetchBegin = existingRange.min;
      }
      if (prefetchEnd < existingRange.max) {
        prefetchEnd = existingRange.max;
      }
    }

    const quantizedBegin = Math.floor(prefetchBegin / 3600) * 3600;
    const quantizedEnd = Math.floor(prefetchEnd / 3600) * 3600;

    let reqTraceIds = [...visibleIds];

    reqTraceIds = Array.from(new Set(reqTraceIds));

    const req: FrameRequest = {
      begin: quantizedBegin,
      end: quantizedEnd,
      trace_ids: reqTraceIds,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };

    const cacheKey = await hashRequest(req, (window as any).perf?.fetch_anomalies_from_sql);
    const db = new TraceDatabase();

    this._summaryLoading = true;
    try {
      const cached = await db.get(cacheKey);
      if (requestId !== this._latestRequestId) return;
      if (cached) {
        if (signal.aborted) return;
        console.log('Prefetch serving from cache:', cacheKey);
        if (cached.dataframe) {
          const newSeries = this._translateDataFrame(cached.dataframe);
          this._processNewSeries(newSeries, false);
        }

        return;
      }

      const response = await DataService.getInstance().sendFrameRequest(req, {
        onProgress: (prog: string) => console.log('Prefetch Progress:', prog),
        onMessage: (msg: string) => console.error('Prefetch Message:', msg),
      });

      if (response && response.dataframe) {
        await db.set(cacheKey, response);
      }

      if (requestId !== this._latestRequestId) {
        console.log('[_prefetchHistory] aborted (but cached): request ID mismatch');
        return;
      }
      if (signal.aborted) {
        console.log('Prefetch aborted');
        return;
      }

      if (response && response.dataframe) {
        const newSeries = this._translateDataFrame(response.dataframe);
        this._processNewSeries(newSeries, false);
      }
    } catch (e) {
      console.error('Prefetch error:', e);
    } finally {
      if (!signal.aborted && requestId === this._latestRequestId) {
        this._summaryLoading = false;
      }
    }
  }

  private _handleToggleRegressions(e: any) {
    this._showRegressions = e.target.checked;
  }

  private _getRegressionCommitBounds(
    reg: Regression | (Regression & Record<string, any>),
    fallbackCommit: number
  ): { start: number; end: number } {
    const rawStart = (reg as any).start_revision ?? reg.prev_commit_number;
    const start =
      rawStart !== undefined && rawStart !== null
        ? Number(rawStart)
        : reg.commit_number !== undefined && reg.commit_number !== null
          ? Number(reg.commit_number) - 1
          : fallbackCommit - 1;
    const end = (reg as any).end_revision ?? reg.commit_number ?? fallbackCommit;
    return { start: Number(start), end: Number(end) };
  }

  /**
   * Calculates the viewport range [minCommit - PADDING, maxCommit + PADDING] covering all
   * (or highlighted) anomalies in _regressions.
   */
  private _getAnomalyViewportRange(): { minCommit: number; maxCommit: number } | null {
    if (!this._regressions || Object.keys(this._regressions).length === 0) {
      return null;
    }

    let minCommit = Infinity;
    let maxCommit = -Infinity;
    const hasHighlightFilter =
      Array.isArray(this.highlightAnomalies) && this.highlightAnomalies.length > 0;

    for (const commitMap of Object.values(this._regressions)) {
      if (!commitMap) continue;
      for (const [commitStr, reg] of Object.entries(commitMap)) {
        if (!reg) continue;
        if (
          hasHighlightFilter &&
          !this.highlightAnomalies.some((id) => String(id) === String(reg.id || ''))
        ) {
          continue;
        }

        const bounds = this._getRegressionCommitBounds(reg, Number(commitStr));
        if (!isNaN(bounds.start) && bounds.start < minCommit) {
          minCommit = bounds.start;
        }
        if (!isNaN(bounds.end) && bounds.end > maxCommit) {
          maxCommit = bounds.end;
        }
      }
    }

    if (minCommit === Infinity || maxCommit === -Infinity) {
      return null;
    }

    return {
      minCommit: Math.max(0, minCommit - ANOMALY_VIEWPORT_PADDING_COMMITS),
      maxCommit: maxCommit + ANOMALY_VIEWPORT_PADDING_COMMITS,
    };
  }

  /**
   * Translates a commit number to the appropriate X-axis viewport coordinate
   * based on the active domain mode (commit number vs. timestamp).
   * Returns null if dateMode is true and timestamp translation fails.
   */
  private _toViewportX(commitNumber: number): number | null {
    if (!this.dateMode) {
      return commitNumber;
    }
    const timestamp = this._translateCommitToTimestamp(commitNumber);
    return timestamp !== UNSET_TIME ? timestamp : null;
  }

  private _onResetZoomEmbedded() {
    const anomalyRange = this._getAnomalyViewportRange();
    if (anomalyRange) {
      const minX = this._toViewportX(anomalyRange.minCommit);
      const maxX = this._toViewportX(anomalyRange.maxCommit);
      if (minX !== null && maxX !== null) {
        this.viewportMinX = minX;
        this.viewportMaxX = maxX;
        return;
      }
    }

    if (this._initialViewportBounds) {
      this.viewportMinX = this._initialViewportBounds.minX;
      this.viewportMaxX = this._initialViewportBounds.maxX;
    } else {
      this.viewportMinX = null;
      this.viewportMaxX = null;
    }
  }

  private _onResetZoom() {
    if (this.embedded) {
      this._onResetZoomEmbedded();
    } else {
      this.viewportMinX = null;
      this.viewportMaxX = null;
      this.begin = UNSET_TIME;
      this.end = UNSET_TIME;
      this._resolveTimeRange();
    }
    this._stateHasChanged();
    this.requestUpdate();
  }

  private _determineYAxisTitle(traceNames: string[]): string {
    if (traceNames.length < 1) {
      return '';
    }

    function parseVal(key: string, traceParams: string[]): string {
      for (const kv of traceParams) {
        if (kv.startsWith(key)) {
          const pieces = kv.split('=', 2);
          return pieces[1];
        }
      }
      return '';
    }

    let idx = 0;
    let params = traceNames[idx].split(',');
    let unit = parseVal('unit', params);
    let improvement_dir = parseVal('improvement_dir', params);

    for (idx = 1; idx < traceNames.length; idx++) {
      params = traceNames[idx].split(',');
      if (unit !== parseVal('unit', params)) {
        unit = '';
      }
      if (improvement_dir !== parseVal('improvement_dir', params)) {
        improvement_dir = '';
      }
      if (unit === '' && improvement_dir === '') {
        return '';
      }
    }

    let title = '';
    if (unit !== '') {
      title += `${unit}`;
    }
    if (improvement_dir !== '') {
      if (unit !== '') {
        title += ' - ';
      }
      title += `${improvement_dir}`;
    }
    return title;
  }

  /**
   * Translates a commit number to its closest corresponding Unix timestamp (createdat)
   * using a high-performance Binary Search closest-match lookup on the sorted series data.
   * Reduces time complexity from O(N) to O(log N).
   *
   * @param commitNumber - The commit number to translate.
   * @returns The resolved Unix timestamp in epoch seconds, or -1 if series data is empty.
   */
  private _translateCommitToTimestamp(commitNumber: number): number {
    if (!this._seriesData || this._seriesData.length === 0) {
      console.log('[_translateCommitToTimestamp] Series data is empty');
      return UNSET_TIME;
    }

    // Find the series with the maximum number of rows (most complete history)
    let series = null;
    let maxRows = 0;
    for (const s of this._seriesData) {
      if (s.rows && s.rows.length > maxRows) {
        maxRows = s.rows.length;
        series = s;
      }
    }

    if (!series) {
      console.log('[_translateCommitToTimestamp] No series with rows found');
      return UNSET_TIME;
    }

    const rows = series.rows;
    if (this._evenXAxisSpacing) {
      if (commitNumber >= 0 && commitNumber < rows.length) {
        return rows[commitNumber].createdat;
      }
      return UNSET_TIME;
    }

    let low = 0;
    let high = rows.length - 1;

    while (low < high) {
      const mid = Math.floor((low + high) / 2);
      if (rows[mid].commit_number === commitNumber) {
        return rows[mid].createdat;
      }
      if (rows[mid].commit_number < commitNumber) {
        low = mid + 1;
      } else {
        high = mid;
      }
    }

    let closestIdx = low;
    if (low > 0) {
      const diff1 = Math.abs(rows[low].commit_number - commitNumber);
      const diff2 = Math.abs(rows[low - 1].commit_number - commitNumber);
      if (diff2 < diff1) {
        closestIdx = low - 1;
      }
    }
    return rows[closestIdx].createdat;
  }

  private _handleViewportChanged(e: any) {
    const detail = e.detail as { minCommit: number | null; maxCommit: number | null };
    const { minCommit, maxCommit } = detail;

    if (minCommit === null || maxCommit === null) {
      this._onResetZoom();
      return;
    }

    // Update viewport instantly for visual sync
    this.viewportMinX = minCommit;
    this.viewportMaxX = maxCommit;

    if (this.dateMode) {
      this.begin = Math.floor(minCommit);
      this.end = Math.ceil(maxCommit);
      this._stateHasChanged();
    } else {
      const beginTime = this._translateCommitToTimestamp(Math.floor(minCommit));
      const endTime = this._translateCommitToTimestamp(Math.ceil(maxCommit));
      let changed = false;
      if (beginTime !== UNSET_TIME && beginTime !== this.begin) {
        this.begin = beginTime;
        changed = true;
      }
      if (endTime !== UNSET_TIME && endTime !== this.end) {
        this.end = endTime;
        changed = true;
      }
      if (changed) {
        this._stateHasChanged();
      }
    }

    if (this._viewportChangeTimeout) {
      clearTimeout(this._viewportChangeTimeout);
    }
    this._viewportChangeTimeout = setTimeout(() => {
      this._doHandleViewportChanged(e).catch((err) => {
        console.error('Failed to handle viewport change:', err);
      });
    }, 300);
  }

  private _handleSummaryRangeSelected(e: CustomEvent<{ begin: number; end: number }>) {
    const { begin, end } = e.detail;
    if (this.viewportMinX === begin && this.viewportMaxX === end) {
      console.log('[_handleSummaryRangeSelected] Viewport matches exactly. Skipped.');
      return;
    }

    this.viewportMinX = begin;
    this.viewportMaxX = end;

    if (this.dateMode) {
      this.begin = Math.floor(begin);
      this.end = Math.ceil(end);
      console.log(
        `[_handleSummaryRangeSelected] DateMode true. New bounds: begin ${this.begin}, end ${this.end}`
      );
      this._stateHasChanged();
    } else {
      const beginTime = this._translateCommitToTimestamp(Math.floor(begin));
      const endTime = this._translateCommitToTimestamp(Math.ceil(end));
      let changed = false;
      if (beginTime !== UNSET_TIME && beginTime !== this.begin) {
        this.begin = beginTime;
        changed = true;
      }
      if (endTime !== UNSET_TIME && endTime !== this.end) {
        this.end = endTime;
        changed = true;
      }
      if (changed) {
        this._stateHasChanged();
      }
    }
  }

  private _handleLoadMore(e: CustomEvent<'left' | 'right'>) {
    const side = e.detail;
    if (side === 'left') {
      this._summaryBeginOffsetSec += SUMMARY_INCREMENT_SEC;
    } else {
      this._summaryEndOffsetSec += SUMMARY_INCREMENT_SEC;
    }
    void this._prefetchHistory(this._latestRequestId);
  }

  private async _doHandleViewportChanged(e: any) {
    const detail = e.detail as { minCommit: number; maxCommit: number };
    const { minCommit, maxCommit } = detail;

    const startIdx = this._tracePage * this._pageSize;
    const endIdx = startIdx + this._pageSize;
    const visibleIds = this._matchingTraceIds.slice(startIdx, endIdx);
    const loadedIds = new Set(this._seriesData.map((s) => s.id));

    const allVisibleIds = [...visibleIds];

    const requests = calculateFetchRequests(
      Array.from(new Set(allVisibleIds)),
      loadedIds,
      { min: minCommit, max: maxCommit },
      this._loadedBounds,
      this._globalBounds,
      this._getPrimaryKey.bind(this),
      this.dateMode
    );
    console.log('[_doHandleViewportChanged] requests:', requests);

    for (const req of requests) {
      try {
        const fetchReq: TraceValuesRequest = {
          ids: req.ids,
          min_commit: 0,
          max_commit: 0,
        };
        if (this.dateMode) {
          fetchReq.begin = Math.floor(req.min || 0);
          fetchReq.end = Math.ceil(req.max || 0);
        } else {
          fetchReq.min_commit = Math.floor(req.min || 0);
          fetchReq.max_commit = Math.ceil(req.max || 0);
        }

        fetchReq.ids.sort();
        const cacheKey = await hashRequest(
          fetchReq,
          (window as any).perf?.fetch_anomalies_from_sql
        );
        const db = new TraceDatabase();
        const cached = await db.get(cacheKey);

        let resp: TraceValuesResponse;
        if (cached) {
          console.log('Serving trace values from cache:', cacheKey);
          resp = cached;
        } else {
          resp = await DataService.getInstance().fetchTraceValues(fetchReq);
          console.log('[_doHandleViewportChanged] fetchTraceValues response:', resp);
          await db.set(cacheKey, resp);
        }

        if (resp && resp.anomalymap) {
          const nextRegressions = { ...this._regressions };
          for (const [traceId, commitMap] of Object.entries(resp.anomalymap)) {
            if (!commitMap) continue;

            const primaryKey = traceId;

            if (!nextRegressions[primaryKey]) {
              nextRegressions[primaryKey] = {};
            }
            for (const [commit, anomaly] of Object.entries(commitMap)) {
              nextRegressions[primaryKey][Number(commit)] = this._mapAnomalyFields(anomaly);
            }
          }
          this._regressions = nextRegressions;
        }

        if (resp && resp.results) {
          const convertedSeries: TraceSeries[] = [];
          for (const [id, rows] of Object.entries(resp.results) as [string, any[]][]) {
            const primaryKey = id;

            let s = convertedSeries.find((cs) => cs.id === primaryKey);
            if (!s) {
              s = { id: primaryKey, rows: [], color: '', allStats: {} };
              convertedSeries.push(s);
            }
            s.originalId = id;

            const mappedRows = rows.map((r) => ({
              commit_number: r.commit_number,
              createdat: r.createdat,
              val: r.val,
              smoothedVal: r.val,
            }));

            s.rows = mappedRows;
          }
          this._mergeSeriesData(convertedSeries);

          // If a left fetch returned no data for an ID, mark that we reached the global min.
          if (req.order === 'DESC') {
            req.ids.forEach((id) => {
              const rows = resp.results[id];
              if (!rows || rows.length === 0) {
                if (!this._globalBounds[id]) {
                  this._globalBounds[id] = { min: Infinity, max: -Infinity };
                }
                const lBounds = this._loadedBounds[id];
                if (lBounds) {
                  this._globalBounds[id].min = lBounds.min;
                }
              }
            });
          }
        }
      } catch (error) {
        console.error('Failed to fetch trace values:', error);
      }
    }
  }

  private _handleToggleTrace(id: string) {
    const nextSeriesData = this._seriesData.map((s) => {
      if (s.id === id) {
        return { ...s, hidden: !s.hidden };
      }
      return s;
    });
    this._updateSeriesData(nextSeriesData);
    this._updateLoadedBounds();
  }

  private _handleCloseChart(ids: string[]) {
    const idSet = new Set(ids);
    this._updateSeriesData(this._seriesData.filter((s) => !idSet.has(s.id)));
    this._updateLoadedBounds();
    ids.forEach((id) => delete this._globalBounds[id]);
  }

  private _mergeSeriesData(olderSeries: TraceSeries[]) {
    const newSeries = [...this._seriesData];
    const map = new Map<string, TraceSeries>();
    newSeries.forEach((s) => map.set(s.id, s));

    olderSeries.forEach((os) => {
      const existing = map.get(os.id);
      if (existing) {
        if (os.originalId) {
          existing.originalId = os.originalId;
        }
        // Merge rows
        const allRows = [...existing.rows, ...os.rows];
        const unique = new Map<number, any>();
        allRows.forEach((r) => unique.set(r.commit_number, r));
        existing.rows = Array.from(unique.values()).sort(
          (a, b) => a.commit_number - b.commit_number
        );
      } else {
        newSeries.push(os);
      }
    });

    this._updateSeriesData(newSeries);
    this._updateLoadedBounds();
  }

  private _translateDataFrame(df: any): TraceSeries[] {
    if (!df || !df.traceset || !df.header) return [];

    const seriesMap = new Map<string, TraceSeries>();
    const keys = Object.keys(df.traceset);
    console.log('[_translateDataFrame] keys count:', keys.length);

    // Cap at 500 traces as requested
    const limitedKeys = keys.slice(0, 500);

    limitedKeys.forEach((key) => {
      const traceValues = df.traceset[key];
      const rows: any[] = [];

      df.header.forEach((header: any, hIdx: number) => {
        const val = traceValues[hIdx];
        const isSentinel = Math.abs(val) > 1e20;
        if (header && val !== null && val !== undefined && !isNaN(val) && !isSentinel) {
          rows.push({
            commit_number: header.offset,
            val: val,
            createdat: header.timestamp,
            hash: header.hash,
            url: header.url,
            author: header.author,
            message: header.message,
          });
        }
      });

      const params = this._parseTraceKey(key);
      const stat = params['stat'];
      console.log('[_translateDataFrame] key:', key, 'stat:', stat);

      const primaryParams = { ...params };
      let primaryKey = key;
      try {
        primaryKey = makeKey(primaryParams);
      } catch (e) {
        console.error('[_translateDataFrame] makeKey failed for', primaryParams, e);
      }

      let s = seriesMap.get(primaryKey);
      if (!s) {
        s = {
          id: primaryKey,
          color: '', // Will assign color later
          rows: rows,
          allStats: {},
          originalId: key,
        };
        seriesMap.set(primaryKey, s);
      }
    });

    console.log('[_translateDataFrame] seriesMap size:', seriesMap.size);

    const result: TraceSeries[] = [];
    let idx = 0;
    seriesMap.forEach((s) => {
      s.color = `hsl(${(idx * 137.5) % 360}, 70%, 50%)`;
      idx++;
      result.push(s);
    });

    return result;
  }

  private _parseTraceKey(key: string): Record<string, string> {
    const params: Record<string, string> = {};
    const parts = key.split(',');
    parts.forEach((part) => {
      if (!part) return;
      const idx = part.indexOf('=');
      if (idx !== -1) {
        const k = part.substring(0, idx);
        const v = part.substring(idx + 1);
        params[k] = v;
      }
    });
    return params;
  }

  private _getPrimaryKey(key: string): string {
    return key;
  }

  private async _toggleV2Mode() {
    const graphConfigs = this.queries.map((q) => {
      const config = new GraphConfig();
      config.queries = [fromParamSet(q)];
      return config;
    });

    const urlParams = new URLSearchParams();
    if (graphConfigs.length > 0) {
      const shortcutId = await updateShortcut(graphConfigs);
      if (shortcutId) {
        urlParams.set('shortcut', shortcutId);
      }
    }
    if (this.splitKeys.size > 0) {
      urlParams.set('splitByKeys', Array.from(this.splitKeys).join(','));
    }
    if (this.dateMode) {
      urlParams.set('dateAxis', 'true');
    }
    if (this.begin && this.begin !== UNSET_TIME) {
      urlParams.set('begin', this.begin.toString());
    }
    if (this.end && this.end !== UNSET_TIME) {
      urlParams.set('end', this.end.toString());
    }

    const nextSearch = urlParams.toString();
    localStorage.setItem('perf:use-explore-v2', 'false');
    this.redirect(`/m${nextSearch ? `?${nextSearch}` : ''}`);
  }

  // Visible for testing
  public redirect(url: string) {
    window.location.href = url;
  }

  private _mergeSeriesWithStats(existing: TraceSeries[], newSeries: TraceSeries[]): TraceSeries[] {
    const existingMap = new Map(existing.map((s) => [s.id, s]));

    newSeries.forEach((s) => {
      const existingSeries = existingMap.get(s.id);
      if (existingSeries) {
        if (s.originalId) {
          existingSeries.originalId = s.originalId;
        }
        if (s.rows && s.rows.length > 0) {
          existingSeries.rows = s.rows;
        }
        if (s.allStats) {
          existingSeries.allStats = { ...existingSeries.allStats, ...s.allStats };
        }
      } else {
        existingMap.set(s.id, s);
      }
    });

    return Array.from(existingMap.values());
  }

  /**
   * Merges new series into state, calculates loaded bounds, and initializes
   * viewport bounds if they are currently unset.
   */
  private _processNewSeries(newSeries: TraceSeries[], updateViewport = true) {
    this._updateSeriesData(this._mergeSeriesWithStats(this._seriesData, newSeries));
    this._updateLoadedBounds();
    if (updateViewport && (this.viewportMinX === null || this.viewportMaxX === null)) {
      const visibleSeries = this._seriesData.filter((s) => !s.hidden);
      const sharedBounds = calculateSharedBounds(visibleSeries, this._globalBounds, this.dateMode);
      if (sharedBounds) {
        const source = Object.keys(sharedBounds)[0];
        this.viewportMinX = sharedBounds[source].min;
        this.viewportMaxX = sharedBounds[source].max;
      }
    }
  }

  private _updateSeriesData(series: TraceSeries[]) {
    this._seriesData = series.map((s, idx) => ({
      ...s,
      color: `hsl(${(idx * 137.5) % 360}, 70%, 50%)`,
    }));
  }

  private _updateLoadedBounds() {
    const visibleSeries = this._seriesData.filter((s) => !s.hidden);
    this._loadedBounds = calculateLoadedBounds(visibleSeries as any, this.dateMode);
  }

  private _handleHoverChanged(e: CustomEvent<{ dataX: number | null }>) {
    this._globalHoverX = e.detail.dataX;
  }

  private _handlePinPoint(e: CustomEvent<{ dataX: number | null }>) {
    this._globalPinnedX = e.detail.dataX;
  }

  private _onAnomalyChanged = (e: Event): void => {
    const detail = (e as CustomEvent).detail;
    if (detail?.anomalies?.length && detail?.traceNames?.length) {
      this._updateRegressionsState(detail.anomalies as Regression[], detail.traceNames);
    }
  };

  private _updateRegressionsState(regressions: Regression[], traceNames: string[]): void {
    const nextRegressions = { ...this._regressions };

    regressions.forEach((reg, index) => {
      if (!reg?.id) return;

      // Use traceNames[0] as fallback when a single trace name is passed for multiple anomalies.
      const traceName = traceNames[index] || traceNames[0];
      if (!traceName) return;

      const commitMap = { ...(nextRegressions[traceName] || {}) };
      this._removeExistingAnomaly(commitMap, reg.id);

      const newCommit = reg.display_commit_number || (reg as any).end_revision;
      if (newCommit) {
        commitMap[newCommit] = this._mapAnomalyFields(reg);
      }

      nextRegressions[traceName] = commitMap;
    });

    this._regressions = nextRegressions;
  }

  private _removeExistingAnomaly(commitMap: Record<number, Regression>, anomalyId: string): void {
    for (const [commitStr, existing] of Object.entries(commitMap)) {
      if (existing && String(existing.id) === String(anomalyId)) {
        delete commitMap[Number(commitStr)];
        break;
      }
    }
  }

  private _onStartTour() {
    this._tourActive = true;
  }

  private _handleTourStepChanged(e: CustomEvent<{ index: number }>) {
    const idx = e.detail.index;
    switch (idx) {
      case 0:
        this._onApplyComparisonPreset();
        break;
      case 1:
        const queryBar = this.shadowRoot!.querySelector('query-bar-sk') as any;
        if (queryBar) {
          const textField = queryBar.shadowRoot.querySelector('.query-input');
          if (textField) {
            textField.dispatchEvent(new Event('focus'));
            textField.value = 'test';
            textField.dispatchEvent(new Event('input', { bubbles: true }));
          }
        }
        break;
      case 2:
        const queryBarForSug = this.shadowRoot!.querySelector('query-bar-sk') as any;
        if (queryBarForSug) {
          const textField = queryBarForSug.shadowRoot.querySelector('.query-input');
          if (textField) {
            textField.dispatchEvent(new Event('focus'));
            textField.dispatchEvent(new Event('input', { bubbles: true }));
          }
        }
        break;
      case 3:
        const qb = this.shadowRoot!.querySelector('query-bar-sk');
        if (qb) {
          const chip = qb.shadowRoot!.querySelector('explore-multi-v2-select-sk');
          if (chip) {
            const trigger = chip.shadowRoot!.querySelector('.multiselect-trigger') as HTMLElement;
            if (trigger) {
              trigger.click();
            }
          }
        }
        break;
      case 4:
        const qb2 = this.shadowRoot!.querySelector('query-bar-sk');
        if (qb2) {
          const chip = qb2.shadowRoot!.querySelector('explore-multi-v2-select-sk');
          if (chip) {
            const trigger = chip.shadowRoot!.querySelector('.multiselect-trigger') as HTMLElement;
            if (trigger) {
              trigger.click();
            }
          }
        }
        break;
      case 6:
        this._handleSplit(new CustomEvent('split', { detail: { key: 'bot' } }));
        break;
      case 7:
        this.dateMode = !this.dateMode;
        this._stateHasChanged();
        break;
      case 8:
        this._smooth = true;
        this._hoverMode = 'both';
        this._smoothingRadius = 50;
        this.requestUpdate();
        break;
      case 10:
        if (this._seriesData.length > 0 && this._seriesData[0].rows.length > 0) {
          this._globalHoverX = this._seriesData[0].rows[0].commit_number;
        }
        this.requestUpdate();
        break;
    }
  }

  private _onApplyRandomPreset() {
    if (!this._workerController) return;
    this._workerController.getRandomTrace((randomQuery) => {
      if (randomQuery) {
        this.queries = [randomQuery];
      }
    });
  }

  private _onApplyComparisonPreset() {
    if (!this._workerController) return;
    this._workerController.getRandomTrace((randomQuery) => {
      if (!randomQuery) return;

      const keys = Object.keys(randomQuery);
      if (keys.length === 0) return;

      // Start from the last key (the most specific parameter from our greedy builder)
      let chosenKey = keys[keys.length - 1];
      let opts = this._optionsByKey[chosenKey] || [];

      // Search backwards to find the most specific parameter that has multiple options to compare
      if (opts.length < 2) {
        for (let i = keys.length - 2; i >= 0; i--) {
          const key = keys[i];
          const options = this._optionsByKey[key] || [];
          if (options.length >= 2) {
            chosenKey = key;
            opts = options;
            break;
          }
        }
      }

      // Fall back to a single trace if no parameter can be compared
      if (opts.length < 2) {
        this.queries = [randomQuery];
        return;
      }

      const val1 = randomQuery[chosenKey][0];
      const otherOpts = opts.filter((o) => o.value !== val1);

      if (otherOpts.length === 0) {
        this.queries = [randomQuery];
        return;
      }

      const val2 = otherOpts[Math.floor(Math.random() * otherOpts.length)].value;

      this.queries = [
        { ...randomQuery, [chosenKey]: [val1] },
        { ...randomQuery, [chosenKey]: [val2] },
      ];
    });
  }

  private _onAddQuery() {
    this.queries = [...this.queries, { ...this._defaultParamSelections }];
    if (this.queries.length > 3) {
      this._queriesExpanded = true;
    }
  }

  private _toggleQueriesExpand() {
    this._queriesExpanded = !this._queriesExpanded;
  }

  private _clearSuggestions(idx: number) {
    const newSuggestions = [...this._suggestionsForQueryBar];
    newSuggestions[idx] = [];
    this._suggestionsForQueryBar = newSuggestions;
  }

  private _onRemoveQueryBar(idx: number) {
    if (this.queries.length > 1) {
      this.queries = this.queries.filter((_, i) => i !== idx);
      this._suggestionsForQueryBar = this._suggestionsForQueryBar.filter((_, i) => i !== idx);
    }
  }

  private _onCloneQueryBar(idx: number) {
    const queryToClone = this.queries[idx];
    const clonedQuery: Record<string, string[]> = {};
    for (const [k, v] of Object.entries(queryToClone)) {
      clonedQuery[k] = [...v];
    }

    const newQueries = [...this.queries];
    newQueries.splice(idx + 1, 0, clonedQuery);
    this.queries = newQueries;

    const newSuggestions = [...this._suggestionsForQueryBar];
    newSuggestions.splice(idx + 1, 0, []);
    this._suggestionsForQueryBar = newSuggestions;

    if (this.queries.length > 3) {
      this._queriesExpanded = true;
    }
  }

  private _handleSuggest(idx: number, e: CustomEvent<{ query: string }>) {
    const queryInput = e.detail.query;
    const currentQuery = this.queries[idx] || {};
    const availableParams = this._availableParamsPerQuery[idx] || this._availableParams;

    const reqId =
      this._workerController?.suggest(queryInput, currentQuery, idx, availableParams) || 0;
    this._latestSuggestRequestIds[idx] = reqId;
  }

  private _applyConditionalDefaults(
    query: Record<string, string[]>,
    changedKey: string,
    newValues: string[],
    oldValues: string[]
  ): Record<string, string[]> {
    if (!this._conditionalDefaults) {
      return query;
    }
    let nextQuery: Record<string, string[]> | null = null;
    for (const rule of this._conditionalDefaults) {
      if (rule.trigger.param === changedKey) {
        const hasNewTrigger = rule.trigger.values.some(
          (v: string) => newValues.includes(v) && !oldValues.includes(v)
        );
        if (hasNewTrigger) {
          if (!nextQuery) {
            nextQuery = { ...query };
          }
          for (const apply of rule.apply) {
            let newValuesToApply = [...apply.values];
            if (apply.select_only_first && newValuesToApply.length > 0) {
              newValuesToApply = [newValuesToApply[0]];
            }
            nextQuery[apply.param] = newValuesToApply;
          }
        }
      }
    }
    return nextQuery || query;
  }

  private _handleAddQuery(idx: number, e: CustomEvent<{ key: string; value: string }>) {
    const { key, value } = e.detail;
    const queries = [...this.queries];
    const current = queries[idx][key] || [];
    if (!current.includes(value)) {
      const newValues = [...current, value];
      queries[idx] = { ...queries[idx], [key]: newValues };
      queries[idx] = this._applyConditionalDefaults(queries[idx], key, newValues, current);
      this.queries = queries;
    }
    this._clearSuggestions(idx);
  }

  private _handleRemoveQuery(idx: number, e: CustomEvent<{ key: string; value: string }>) {
    const { key, value } = e.detail;
    const queries = [...this.queries];
    const current = queries[idx][key] || [];
    const updated = current.filter((v) => v !== value);
    if (updated.length === 0) {
      const nextQuery = { ...queries[idx] };
      delete nextQuery[key];
      queries[idx] = nextQuery;
    } else {
      queries[idx] = { ...queries[idx], [key]: updated };
    }
    this.queries = queries;
    this._clearSuggestions(idx);
  }

  private _handleSetSelected(idx: number, e: CustomEvent<{ key: string; values: string[] }>) {
    const { key, values } = e.detail;
    const queries = [...this.queries];
    const oldValues = queries[idx][key] || [];
    queries[idx] = { ...queries[idx], [key]: values };
    queries[idx] = this._applyConditionalDefaults(queries[idx], key, values, oldValues);
    this.queries = queries;
    this._clearSuggestions(idx);
  }

  private _handleRemoveKey(idx: number, e: CustomEvent<{ key: string }>) {
    if (!e.detail) {
      const queries = [...this.queries];
      queries[idx] = {};
      this.queries = queries;
      this._clearSuggestions(idx);
      return;
    }
    const { key } = e.detail;
    const queries = [...this.queries];
    const nextQuery = { ...queries[idx] };
    delete nextQuery[key];
    queries[idx] = nextQuery;
    this.queries = queries;
    this._clearSuggestions(idx);
  }

  private _handleSplit(e: CustomEvent<{ key: string }>) {
    const { key } = e.detail;
    const nextSplit = new Set(this.splitKeys);
    if (nextSplit.has(key)) {
      nextSplit.delete(key);
    } else {
      nextSplit.add(key);
    }
    this.splitKeys = nextSplit;
  }

  private _handleReorderSplitKeys(e: CustomEvent<{ keys: string[] }>) {
    this.splitKeys = new Set(e.detail.keys);
  }

  private _handleDiffBase(e: CustomEvent<{ key: string; value: string }>) {
    const { key, value } = e.detail;
    console.log('[_handleDiffBase] Received event. key:', key, 'value:', value);
    if (this._diffBase && this._diffBase.key === key && this._diffBase.value === value) {
      console.log('[_handleDiffBase] Clearing diffBase');
      this._diffBase = null;
    } else {
      console.log('[_handleDiffBase] Setting diffBase to:', { key, value });
      this._diffBase = { key, value };
    }
  }

  private async _fetchMetadataForVisibleTraces(requestId: number) {
    if (requestId !== this._latestRequestId) return;
    if (!this._seriesData || this._seriesData.length === 0) return;

    const startIdx = this._tracePage * this._pageSize;
    const endIdx = startIdx + this._pageSize;
    const currentVisibleIds = new Set(
      this._matchingTraceIds.slice(startIdx, endIdx).map((id) => this._getPrimaryKey(id))
    );

    const visibleSeries = this._seriesData.filter((s) => currentVisibleIds.has(s.id));
    if (visibleSeries.length === 0) return;

    const traceIdToOriginalId = new Map<string, string>();
    const traceIds = visibleSeries.map((s) => {
      const tId = s.originalId || s.id;
      traceIdToOriginalId.set(s.id, tId);
      return tId;
    });
    const commitNumbersSet = new Set<number>();

    visibleSeries.forEach((s) => {
      s.rows.forEach((r) => {
        if (r.metadata === undefined && !this._inFlightMetadataCommits.has(r.commit_number)) {
          commitNumbersSet.add(r.commit_number);
        }
      });
    });

    const commitNumbers = Array.from(commitNumbersSet);
    if (commitNumbers.length === 0) {
      return;
    }

    commitNumbers.forEach((c) => this._inFlightMetadataCommits.add(c));

    try {
      console.log(
        `Fetching metadata for ${traceIds.length} traces and ${commitNumbers.length} commits.`
      );

      const db = new TraceDatabase();
      const cacheKey = await hashRequest({ commitNumbers, traceIds });
      const cached = await db.get(cacheKey);
      if (requestId !== this._latestRequestId) return;

      let metadataResp: any;
      if (cached) {
        console.log('Serving metadata from cache:', cacheKey);
        metadataResp = cached;
      } else {
        metadataResp = await DataService.getInstance().getLinksBatch(commitNumbers, traceIds);
        if (requestId !== this._latestRequestId) return;
        await db.set(cacheKey, metadataResp);
      }
      if (requestId !== this._latestRequestId) return;

      const nextSeriesData = [...this._seriesData];
      let updatedCount = 0;

      nextSeriesData.forEach((s, idx) => {
        if (currentVisibleIds.has(s.id)) {
          const nextRows = [...s.rows];
          let rowChanged = false;
          const requestedId = traceIdToOriginalId.get(s.id) || s.id;
          nextRows.forEach((r, rowIdx) => {
            if (commitNumbersSet.has(r.commit_number) && r.metadata === undefined) {
              const commitMetadataMap = metadataResp?.[r.commit_number.toString()] || {};
              let commitMetadata: any = null;
              for (const [respTraceId, links] of Object.entries(commitMetadataMap)) {
                if (
                  this._getPrimaryKey(respTraceId) === this._getPrimaryKey(requestedId) ||
                  this._getPrimaryKey(respTraceId) === this._getPrimaryKey(s.id) ||
                  this._getPrimaryKey(respTraceId) === this._getPrimaryKey(s.originalId || '')
                ) {
                  commitMetadata = links;
                  break;
                }
              }
              if (!commitMetadata) {
                commitMetadata =
                  commitMetadataMap[requestedId] ||
                  commitMetadataMap[s.id] ||
                  commitMetadataMap[s.originalId || ''] ||
                  null;
              }
              nextRows[rowIdx] = { ...r, metadata: commitMetadata };
              rowChanged = true;
              updatedCount++;
            }
          });
          if (rowChanged) {
            nextSeriesData[idx] = { ...s, rows: nextRows };
          }
        }
      });

      if (updatedCount > 0) {
        console.log(`Successfully fetched and attached metadata to ${updatedCount} trace rows.`);
        this._updateSeriesData(nextSeriesData);
        this._updateAvailableSubrepos();
      }

      // Clear in-flight requests since they are either fulfilled or marked as null
      commitNumbers.forEach((c) => this._inFlightMetadataCommits.delete(c));
    } catch (e) {
      console.error('Failed to fetch metadata:', e);
      // On failure, remove from in-flight so we can retry later
      commitNumbers.forEach((c) => this._inFlightMetadataCommits.delete(c));
    }
  }

  private _updateAvailableSubrepos() {
    const keys = new Set<string>();
    this._seriesData.forEach((s) => {
      s.rows.forEach((r: any) => {
        if (r.metadata) {
          Object.keys(r.metadata).forEach((k) => {
            if (SUBREPO_CONFIG[k]) {
              keys.add(k);
            }
          });
        }
      });
    });
    this._availableSubrepos = Array.from(keys).sort();
  }

  private _handleControlChange(e: CustomEvent<{ name: string; value: any }>) {
    const { name, value } = e.detail;
    if (name in this) {
      (this as any)[name] = value;
    } else {
      (this as any)[`_${name}`] = value;
    }

    // Handle side effects
    if (name === 'dateMode') {
      this._globalBounds = {};
      this._updateLoadedBounds();
      this.viewportMinX = null;
      this.viewportMaxX = null;
    } else if (name === 'smooth') {
      this._hoverMode = value ? 'both' : 'original';
    }
  }

  private get _availableSplitKeys(): string[] {
    const allKeys = Object.keys(this._optionsByKey);
    return allKeys.filter((k) => !this._includeParams.includes(k));
  }

  render() {
    let displaySeries = this._diffBase
      ? computeTraceDiffs(this._seriesData, this._diffBase)
      : this._seriesData;

    if (this._transformPreset !== 'none') {
      displaySeries = computeCustomTransforms(displaySeries, this._transformPreset);
    }

    const totalMatchedPages = Math.max(
      1,
      Math.ceil(this._matchingTraceIds.length / this._pageSize)
    );
    const clampedPage = Math.max(0, Math.min(this._tracePage, totalMatchedPages - 1));

    const startIdx = clampedPage * this._pageSize;
    const endIdx = startIdx + this._pageSize;
    const currentVisibleIds = this._showAllTraces
      ? new Set(this._matchingTraceIds.slice(0, 500).map((id) => this._getPrimaryKey(id)))
      : new Set(
          this._matchingTraceIds.slice(startIdx, endIdx).map((id) => this._getPrimaryKey(id))
        );

    const currentPageTraces = displaySeries.filter((s) => {
      // Note: We cannot simply use `(s as any).originalId || s.id`.
      // For original series, `s.id` is the collapsed primary key (e.g. without `stat=min/max`),
      // while `s.originalId` is the un-collapsed raw database key (containing `stat=`).
      // Since `currentVisibleIds` only contains collapsed primary keys, using `originalId` on
      // original traces would check for un-collapsed keys and discard all traces from the graph.
      // Therefore, we only fallback to `originalId` if the series is a transformed series.
      const primaryKey = s.id.includes('special=transform') ? (s as any).originalId : s.id;
      return currentVisibleIds.has(primaryKey);
    });
    const groups = computeSplitGroups(currentPageTraces, this.splitKeys);

    return html`
      ${this.embedded
        ? ''
        : html`
            <div class="header">
              <div>
                <h1>Explore Multi V2</h1>
                <p class="subtitle">
                  High-performance custom dimension analysis (Work in Progress)
                </p>
              </div>
              <div
                class="v2-toggle-container"
                style="display: inline-flex; align-items: center; gap: 12px; border-radius: 8px; padding: 8px 16px; border: 1px solid var(--outline, rgba(255,255,255,0.1)); background-color: rgba(128,128,128,0.05);">
                <span
                  style="font-size: 12px; font-weight: 600; color: var(--on-background, #cbd5e1);"
                  >Explore Multi V2:</span
                >
                <button
                  @click=${this._toggleV2Mode}
                  style="background: var(--primary); color: var(--on-primary); border: none; padding: 4px 16px; border-radius: 12px; font-size: 11px; font-weight: bold; cursor: pointer; transition: all 0.2s;">
                  ACTIVE (Switch to Legacy)
                </button>
              </div>
            </div>
          `}

      <div class="workspace">
        <div class="section-title">Faceted Search Bar</div>
        ${this.queries.map((q, idx) => {
          if (!this._queriesExpanded && idx >= 3) {
            return '';
          }
          return html`
            <div class="query-row" style="display: flex; align-items: center; gap: 8px;">
              <query-bar-sk
                style="flex: 1;"
                .query=${q}
                .availableParams=${this._availableParamsPerQuery[idx] &&
                this._availableParamsPerQuery[idx].length > 0
                  ? this._availableParamsPerQuery[idx]
                  : this._availableParams}
                .optionsByKey=${this._optionsByKeyPerQuery[idx] &&
                Object.keys(this._optionsByKeyPerQuery[idx]).length > 0
                  ? this._optionsByKeyPerQuery[idx]
                  : this._optionsByKey}
                .splitKeys=${this.splitKeys}
                .includeParams=${this._includeParams}
                .defaults=${this._defaults}
                .showRemoveQueryButton=${this.queries.length > 1}
                .externalSuggestions=${this._suggestionsForQueryBar[idx] || null}
                @suggest=${(e: CustomEvent) => this._handleSuggest(idx, e)}
                @add-query=${(e: CustomEvent) => this._handleAddQuery(idx, e)}
                @remove-query=${(e: CustomEvent) => this._handleRemoveQuery(idx, e)}
                @set-selected=${(e: CustomEvent) => this._handleSetSelected(idx, e)}
                @remove-key=${(e: CustomEvent) => this._handleRemoveKey(idx, e)}
                @split=${(e: CustomEvent) => this._handleSplit(e)}
                @diff-base=${(e: CustomEvent) => this._handleDiffBase(e)}
                @clear-query=${() => this._onRemoveQueryBar(idx)}
                @clone-query=${() => this._onCloneQueryBar(idx)}></query-bar-sk>
            </div>
          `;
        })}
        <div
          style="display: flex; justify-content: center; align-items: center; gap: 8px; margin-top: -6px; position: relative; z-index: 1;">
          <button class="add-query-circle-btn" @click=${this._onAddQuery} title="Add Query">
            +
          </button>
          ${this.queries.length > 3
            ? html`
                <button
                  class="expand-queries-btn"
                  @click=${this._toggleQueriesExpand}
                  title="${this._queriesExpanded
                    ? 'Hide extra search bars'
                    : 'Show all search bars'}">
                  ${this._queriesExpanded ? 'Collapse' : `Expand (${this.queries.length - 3} more)`}
                </button>
              `
            : ''}
        </div>

        ${this._diffBase || this.splitKeys.size > 0
          ? html`
              <div class="config-pills">
                ${Array.from(this.splitKeys).map(
                  (key) => html`
                    <div class="config-pill">
                      <span class="config-pill-label">Split by:</span>
                      <span>${key}</span>
                      <button
                        class="config-pill-remove"
                        @click=${() =>
                          this._handleSplit(new CustomEvent('split', { detail: { key } }))}>
                        &times;
                      </button>
                    </div>
                  `
                )}
                ${this._diffBase
                  ? html`
                      <div class="config-pill diff-base">
                        <span class="config-pill-label">Diff Base:</span>
                        <span
                          class="config-pill-value"
                          title=${`${this._diffBase.key}=${this._diffBase.value}`}>
                          ${this._diffBase.value}
                        </span>
                        <button
                          class="config-pill-remove"
                          @click=${() =>
                            this._handleDiffBase(
                              new CustomEvent('diff-base', {
                                detail: { key: this._diffBase!.key, value: this._diffBase!.value },
                              })
                            )}>
                          &times;
                        </button>
                      </div>
                    `
                  : ''}
              </div>
            `
          : ''}

        <explore-toolbar-sk
          .tracePage=${this._tracePage}
          .totalMatchedPages=${totalMatchedPages}
          .showAllTraces=${this._showAllTraces}
          .selectedSubrepo=${this._selectedSubrepo}
          .availableSubrepos=${this._availableSubrepos}
          .normalizeCentre=${this._normalizeCentre}
          .smooth=${this._smooth}
          .showDots=${this._showDots}
          .showSparklines=${this._showSparklines}
          .evenXAxisSpacing=${this._evenXAxisSpacing}
          .transformPreset=${this._transformPreset}
          .showRegressions=${this._showRegressions}
          .tooltipDiffs=${this._tooltipDiffs}
          .dateMode=${this.dateMode}
          .hoverMode=${this._hoverMode}
          .smoothingRadius=${this._smoothingRadius}
          .edgeDetectionFactor=${this._edgeDetectionFactor}
          .edgeLookahead=${this._edgeLookahead}
          .availableSplitKeys=${this._availableSplitKeys}
          .activeSplitKeys=${Array.from(this.splitKeys)}
          .pageSize=${this._pageSize}
          @control-change=${this._handleControlChange}
          @split=${this._handleSplit}
          @reset-zoom=${this._onResetZoom}></explore-toolbar-sk>

        <div class="section-title">Visualizations</div>

        ${this._loading && groups.length === 0
          ? html`
              <div class="page-loading">
                <div class="spinner"></div>
                <span>${this._statusMessage || 'Loading traces...'}</span>
              </div>
            `
          : ''}

        <div class="${this._showSparklines ? 'sparklines-grid' : 'charts-container'}">
          ${groups.map(
            (g) => html`
              <trace-chart-sk
                .title=${g.title}
                .canvasHeight=${this._showSparklines ? 150 : 300}
                .isSparkline=${this._showSparklines}
                .loading=${this._loading}
                .series=${g.series}
                .dateMode=${this.dateMode}
                .regressions=${this._showRegressions ? this._regressions : {}}
                .normalizeCentre=${this._normalizeCentre}
                .normalizeScale=${this._normalizeScale}
                .hoverMode=${this._hoverMode}
                .smoothingRadius=${this._smoothingRadius}
                .edgeDetectionFactor=${this._edgeDetectionFactor}
                .edgeLookahead=${this._edgeLookahead}
                .showDots=${this._showDots}
                .evenXAxisSpacing=${this._evenXAxisSpacing}
                .viewportMinX=${this.viewportMinX}
                .viewportMaxX=${this.viewportMaxX}
                .globalHoverX=${this._globalHoverX}
                .globalPinnedX=${this._globalPinnedX}
                .loadedBounds=${this._loadedBounds}
                .globalBounds=${this._globalBounds}
                .highlightAnomalies=${this.highlightAnomalies}
                .tooltipDiffs=${this._tooltipDiffs}
                .selectedSubrepo=${this._selectedSubrepo}
                .activeSplitKeys=${Array.from(this.splitKeys)}
                .user_id=${this._user}
                .yAxisLabel=${this._determineYAxisTitle(g.series.map((s) => s.id))}
                @viewport-changed=${this._handleViewportChanged}
                @hover-changed=${this._handleHoverChanged}
                @pin-point=${this._handlePinPoint}
                @toggle-split=${this._handleSplit}
                @reorder-split-keys=${this._handleReorderSplitKeys}
                @toggle-trace=${(e: CustomEvent<{ id: string }>) =>
                  this._handleToggleTrace(e.detail.id)}
                @close-chart=${() => this._handleCloseChart(g.series.map((s) => s.id))}>
                <plot-summary-v2-sk
                  slot="summary"
                  ?hidden=${!this._showSummaryBar}
                  .series=${g.series}
                  .domain=${this.dateMode ? 'date' : 'commit'}
                  .viewportMinX=${this.viewportMinX}
                  .viewportMaxX=${this.viewportMaxX}
                  .evenXAxisSpacing=${this._evenXAxisSpacing}
                  .loading=${this._summaryLoading}
                  @summary-range-selected=${this._handleSummaryRangeSelected}
                  @load-more-click=${this._handleLoadMore}>
                </plot-summary-v2-sk>
              </trace-chart-sk>
            `
          )}
        </div>
      </div>

      <help-hub-sk
        @start-tour=${this._onStartTour}
        @request-random-preset=${this._onApplyRandomPreset}
        @request-comparison-preset=${this._onApplyComparisonPreset}></help-hub-sk>

      <interactive-tour-sk
        .active=${this._tourActive}
        .steps=${this._tourSteps}
        @step-changed=${this._handleTourStepChanged}
        @tour-finished=${() => {
          this._tourActive = false;
        }}></interactive-tour-sk>

      ${this._workerInitializing
        ? html`
            <div class="worker-init-overlay">
              <div class="spinner"></div>
              <div class="worker-init-status">${this._statusMessage}</div>
            </div>
          `
        : ''}
    `;
  }
}
