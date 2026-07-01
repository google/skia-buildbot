import './explore-multi-v2-sk';
import { ExploreMultiV2Sk } from './explore-multi-v2-sk';
import { UNSET_TIME } from '../const/const';
import { expect } from 'chai';
import sinon from 'sinon';
import { DataService } from '../data-service';
import { TraceDatabase } from './db';

describe('explore-multi-v2-sk', () => {
  let element: ExploreMultiV2Sk;
  let globalOldSubtle: any;
  let globalOldGet: any;
  let globalOldSet: any;

  beforeEach(async () => {
    window.history.replaceState(null, '', window.location.pathname);
    (window as any).WORKER_URL =
      'data:application/javascript,self.postMessage({ type: "LOADED" }); self.onmessage = (e) => { if (e.data.type === "INIT") { self.postMessage({ type: "READY" }); } };';
    element = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;
    element['_debounceDelay'] = 0;
    (element as any)._fetchMetadata = async () => {};
    document.body.appendChild(element);
    await element.updateComplete;

    // Mock crypto.subtle for hashRequest
    globalOldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    // Mock TraceDatabase to avoid real IndexedDB calls
    globalOldGet = TraceDatabase.prototype.get;
    globalOldSet = TraceDatabase.prototype.set;
    TraceDatabase.prototype.get = async () => null; // Default to cache miss
    TraceDatabase.prototype.set = async () => {};
  });

  afterEach(() => {
    document.body.removeChild(element);
    // Restore mocks
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => globalOldSubtle,
      configurable: true,
    });
    TraceDatabase.prototype.get = globalOldGet;
    TraceDatabase.prototype.set = globalOldSet;
  });

  it('uses custom selects', () => {
    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    const selects = toolbar!.shadowRoot!.querySelectorAll('select.custom-select');
    expect(selects.length).to.be.greaterThan(0);
  });

  it('uses custom checkboxes', () => {
    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    const checkboxes = toolbar!.shadowRoot!.querySelectorAll(
      '.custom-checkbox input[type="checkbox"]'
    );
    expect(checkboxes.length).to.be.greaterThan(0);
  });

  it('uses custom sliders for numeric inputs', async () => {
    element['_hoverMode'] = 'smoothed';
    await element.updateComplete;
    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    await (toolbar as any).updateComplete;
    const sliders = toolbar!.shadowRoot!.querySelectorAll('input.custom-slider[type="range"]');
    expect(sliders.length).to.be.greaterThan(0);
  });

  it('updates edge slider max and adds outlier slider', async () => {
    element['_hoverMode'] = 'smoothed';
    await element.updateComplete;
    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    await (toolbar as any).updateComplete;

    const edgeSlider = toolbar!.shadowRoot!.querySelector(
      'input.custom-slider[type="range"][max="1"]'
    );
    expect(edgeSlider).to.not.be.null;

    const outlierSlider = toolbar!.shadowRoot!.querySelector(
      'input.custom-slider[type="range"][max="5"]'
    );
    expect(outlierSlider).to.not.be.null;
  });

  it('uses custom buttons', async () => {
    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    await (toolbar as any).updateComplete;
    const buttons = toolbar!.shadowRoot!.querySelectorAll('button.custom-btn');
    expect(buttons.length).to.be.greaterThan(0);
  });

  it('smooth is unchecked by default', () => {
    expect((element as any)._smooth).to.be.false;
  });

  it('syncs all toolbar checkboxes to URL', async () => {
    (element as any)._showSparklines = true;
    (element as any)._showRegressions = false;
    (element as any)._tooltipDiffs = true;
    (element as any).dateMode = true;
    (element as any)._tracePage = 3;
    (element as any)._pageSize = 25;
    (element as any)._showAllTraces = true;
    (element as any)._selectedSubrepo = 'Skia';
    (element as any)._edgeDetectionFactor = 0.75;
    (element as any)._edgeLookahead = 4;

    (element as any)._stateHasChanged();
    await new Promise((resolve) => setTimeout(resolve, 0));

    const url = new URL(window.location.href);
    expect(url.searchParams.get('sparklines')).to.equal('true');
    expect(url.searchParams.get('regressions')).to.equal('false');
    expect(url.searchParams.get('tooltipDiffs')).to.equal('true');
    expect(url.searchParams.get('dateMode')).to.equal('true');
    expect(url.searchParams.get('page')).to.equal('3');
    expect(url.searchParams.get('pageSize')).to.equal('25');
    expect(url.searchParams.get('showAll')).to.equal('true');
    expect(url.searchParams.get('subrepo')).to.equal('Skia');
    expect(url.searchParams.get('edgeFactor')).to.equal('0.75');
    expect(url.searchParams.get('outlier')).to.equal('4');
  });

  it('triggers state update on dateMode, page, pageSize, showAll, subrepo, edgeFactor or outlier changes', async () => {
    let stateChangedCalled = false;
    (element as any)._stateHasChanged = () => {
      stateChangedCalled = true;
    };

    stateChangedCalled = false;
    element['dateMode'] = true;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_tracePage'] = 2;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_pageSize'] = 50;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_showAllTraces'] = true;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_selectedSubrepo'] = 'V8';
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_edgeDetectionFactor'] = 0.8;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;

    stateChangedCalled = false;
    element['_edgeLookahead'] = 2;
    await element.updateComplete;
    expect(stateChangedCalled).to.be.true;
  });

  it('syncs begin and end to URL', async () => {
    (element as any).begin = 1680000000;
    (element as any).end = 1680100000;

    (element as any)._stateHasChanged();
    await new Promise((resolve) => setTimeout(resolve, 0));

    const url = new URL(window.location.href);
    expect(url.searchParams.get('begin')).to.equal('1680000000');
    expect(url.searchParams.get('end')).to.equal('1680100000');
  });

  it('resolves time range correctly using explicit bounds', () => {
    (element as any).begin = 1680000000;
    (element as any).end = 1680100000;
    const range1 = (element as any)._resolveTimeRange();
    expect(range1.begin).to.equal(1680000000);
    expect(range1.end).to.equal(1680100000);
  });

  it('immediately writes back resolved default bounds to keep state deterministic', () => {
    (element as any).begin = UNSET_TIME;
    (element as any).end = UNSET_TIME;

    const range = (element as any)._resolveTimeRange();
    expect((element as any).begin).to.equal(range.begin);
    expect((element as any).end).to.equal(range.end);
    expect((element as any).begin).to.not.equal(UNSET_TIME);
    expect((element as any).end).to.not.equal(UNSET_TIME);
  });

  it('syncs dateMode to URL', async () => {
    (element as any).dateMode = true;

    (element as any)._stateHasChanged();
    await new Promise((resolve) => setTimeout(resolve, 0));

    const url = new URL(window.location.href);
    expect(url.searchParams.get('dateMode')).to.equal('true');
  });

  it('deserializes dateMode from URL', async () => {
    // Clear state
    window.history.replaceState(null, '', window.location.pathname);

    // Set dateMode in URL
    const url = new URL(window.location.href);
    url.searchParams.set('dateMode', 'true');
    window.history.pushState({}, '', url.toString());

    // Create a new element to see if it picks up the state from URL
    const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;
    document.body.appendChild(newElement);
    await newElement.updateComplete;

    expect((newElement as any).dateMode).to.be.true;
    document.body.removeChild(newElement);
  });
  it('sends all queries to worker', async () => {
    let filterArgs: any = null;
    const mockController = {
      isReady: () => true,
      filter: (queries: any, numUserQueries: number, requestId?: number) => {
        filterArgs = { queries, numUserQueries, requestId };
        return requestId || 0;
      },
      terminate: () => {},
    };
    (element as any)['_workerController'] = mockController;

    element['queries'] = [{ bot: ['linux32'], test: ['binary_size'] }, { bot: ['mac64'] }];

    element['_triggerWorkerFilter']();

    expect(filterArgs).to.not.be.null;
    expect(filterArgs.queries.length).to.equal(4); // 2 user queries + 2 facet removed queries
    expect(filterArgs.numUserQueries).to.equal(2);
  });

  it('does not set globalBounds to loadedBounds on trace selection', async () => {
    const mockDataService = {
      getLinksBatch: async () => ({}),
      sendFrameRequest: async () => ({
        dataframe: {
          header: [{ offset: 10, timestamp: 1000 }],
          traceset: { t1: [1.0] },
        },
      }),
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    const oldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    const oldGet = TraceDatabase.prototype.get;
    const oldSet = TraceDatabase.prototype.set;
    TraceDatabase.prototype.get = async () => null;
    TraceDatabase.prototype.set = async () => {};

    try {
      element['_seriesData'] = [];
      element['_matchingTraceIds'] = ['t1'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;

      await element['_fetchData'](element['_latestRequestId']);

      expect(element['_globalBounds']).to.deep.equal({});
    } finally {
      (DataService as any).instance = oldInstance;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => oldSubtle,
        configurable: true,
      });
      TraceDatabase.prototype.get = oldGet;
      TraceDatabase.prototype.set = oldSet;
    }
  });

  it('calls fetchTraceValues with integer commits when given floats', async () => {
    let fetchTraceValuesArg: any = null;
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (arg: any) => {
        fetchTraceValuesArg = arg;
        return { results: {} };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    try {
      element['_matchingTraceIds'] = ['t1'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [{ id: 't1', rows: [], color: '#fff' }];
      element['_loadedBounds'] = { t1: { min: 100000, max: 104000 } };
      element['_globalBounds'] = { t1: { min: 90000, max: 110000 } };

      await element['_doHandleViewportChanged']({
        detail: { minCommit: 103700.01463834244, maxCommit: 105000.5 },
      });

      expect(fetchTraceValuesArg).to.not.be.null;
      expect(Number.isInteger(fetchTraceValuesArg.min_commit)).to.be.true;
      expect(Number.isInteger(fetchTraceValuesArg.max_commit)).to.be.true;
    } finally {
      (DataService as any).instance = oldInstance;
    }
  });

  it('calls fetchTraceValues with begin and end in Date Mode', async () => {
    let fetchTraceValuesArg: any = null;
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (arg: any) => {
        fetchTraceValuesArg = arg;
        return { results: {} };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    try {
      element['_matchingTraceIds'] = ['t1'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [];
      element['_loadedBounds'] = {};
      element['_globalBounds'] = {};
      element['dateMode'] = true;

      await element['_doHandleViewportChanged']({
        detail: { minCommit: 1700000000.5, maxCommit: 1700005000.7 },
      });

      expect(fetchTraceValuesArg).to.not.be.null;
      expect(fetchTraceValuesArg.begin).to.equal(1700000000);
      expect(fetchTraceValuesArg.end).to.equal(1700005001);
      expect(fetchTraceValuesArg.min_commit).to.equal(0);
      expect(fetchTraceValuesArg.max_commit).to.equal(0);
    } finally {
      (DataService as any).instance = oldInstance;
    }
  });

  it('updates globalBounds when fetch returns no new data on the left', async () => {
    let fetchCalled = false;
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (_arg: any) => {
        fetchCalled = true;
        return { results: { t1: [] } }; // Return empty rows for t1
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    try {
      element['_matchingTraceIds'] = ['t1'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [
        {
          id: 't1',
          rows: [{ commit_number: 1000, createdat: 0, val: 1, smoothedVal: 1 }],
          color: '#fff',
        },
      ];
      element['_loadedBounds'] = { t1: { min: 1000, max: 2000 } };
      element['_globalBounds'] = {}; // Empty global bounds

      // Trigger viewport change that requires left fetch
      await element['_doHandleViewportChanged']({
        detail: { minCommit: 500, maxCommit: 1500 },
      });

      expect(fetchCalled).to.be.true;
      // Since it returned no new data, globalBounds.min should be set to loadedBounds.min
      expect(element['_globalBounds']['t1']).to.not.be.undefined;
      expect(element['_globalBounds']['t1'].min).to.equal(1000);
    } finally {
      (DataService as any).instance = oldInstance;
    }
  });

  it('determines Y axis title correctly', () => {
    const title1 = element['_determineYAxisTitle'](['benchmark=A,unit=ms', 'benchmark=B,unit=ms']);
    expect(title1).to.equal('ms');

    const title2 = element['_determineYAxisTitle'](['benchmark=A,unit=ms', 'benchmark=B,unit=s']);
    expect(title2).to.equal('');

    const title3 = element['_determineYAxisTitle']([
      'benchmark=A,unit=ms,improvement_dir=up',
      'benchmark=B,unit=ms,improvement_dir=up',
    ]);
    expect(title3).to.equal('ms - up');

    const title4 = element['_determineYAxisTitle']([]);
    expect(title4).to.equal('');
  });

  it('returns primary key exactly as given without removing stat', () => {
    const key1 = ',benchmark=A,stat=min,test=B,';
    const primary1 = element['_getPrimaryKey'](key1);
    expect(primary1).to.equal(',benchmark=A,stat=min,test=B,');

    const key2 = ',benchmark=A,test=B,';
    const primary2 = element['_getPrimaryKey'](key2);
    expect(primary2).to.equal(',benchmark=A,test=B,');
  });

  it('translates traceset keys into independent series without grouping', () => {
    const df = {
      header: [{ offset: 10, timestamp: 1000 }],
      traceset: {
        ',benchmark=A,test=B,': [1.0],
        ',benchmark=A,test=B,stat=min,': [0.5],
        ',benchmark=A,test=B,stat=max,': [1.5],
      },
    };

    const series = element['_translateDataFrame'](df);

    expect(series.length).to.equal(3);
    expect(series.map((s: any) => s.id)).to.deep.equal([
      ',benchmark=A,test=B,',
      ',benchmark=A,stat=min,test=B,',
      ',benchmark=A,stat=max,test=B,',
    ]);
    expect(series[0].rows[0].val).to.equal(1.0);
    expect(series[1].rows[0].val).to.equal(0.5);
    expect(series[2].rows[0].val).to.equal(1.5);
  });

  it('merges series data correctly accumulating stats', () => {
    const existing = [
      {
        id: ',benchmark=A,test=B,',
        color: 'red',
        rows: [{ commit_number: 10, val: 1.0 }],
        allStats: {},
      },
    ];

    const newSeries = [
      {
        id: ',benchmark=A,test=B,',
        color: 'blue',
        rows: [],
        allStats: { min: [{ commit_number: 10, val: 0.5 }] },
      },
    ];

    const merged = (element as any)['_mergeSeriesWithStats'](existing, newSeries);

    expect(merged.length).to.equal(1);
    expect(merged[0].rows.length).to.equal(1);
    expect(merged[0].rows[0].val).to.equal(1.0); // Kept existing rows
    expect(merged[0].allStats['min']).to.not.be.undefined;
    expect(merged[0].allStats['min'][0].val).to.equal(0.5); // Accumulated stats
  });

  it('toggles showAllTraces when Show All button is clicked', async () => {
    element['_showAllTraces'] = false;
    await element.updateComplete;

    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    const showAllBtn = Array.from(toolbar!.shadowRoot!.querySelectorAll('button')).find((b) =>
      b.textContent?.trim().includes('Show All')
    );
    expect(showAllBtn).to.not.be.undefined;

    showAllBtn!.click();
    await element.updateComplete;

    expect(element['_showAllTraces']).to.be.true;
    expect(showAllBtn!.textContent?.trim()).to.include('Show Paged');

    showAllBtn!.click();
    await element.updateComplete;

    expect(element['_showAllTraces']).to.be.false;
    expect(showAllBtn!.textContent?.trim()).to.include('Show All');
  });

  it('triggers fetch and fetches all traces when showAllTraces changes', async () => {
    let fetchCount = 0;
    let fetchTraceIdsArg: string[] = [];
    const mockDataService = {
      getLinksBatch: async () => ({}),
      sendFrameRequest: async (req: any) => {
        fetchCount++;
        fetchTraceIdsArg = req.trace_ids;
        return {
          dataframe: {
            header: [{ offset: 10, timestamp: 1000 }],
            traceset: { t1: [1.0] },
          },
        };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    const oldGet = TraceDatabase.prototype.get;
    const oldSet = TraceDatabase.prototype.set;
    TraceDatabase.prototype.get = async () => null;
    TraceDatabase.prototype.set = async () => {};

    const oldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    try {
      element['_matchingTraceIds'] = Array.from({ length: 20 }, (_, i) => `t${i}`);
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [];

      await element.updateComplete;

      await element['_fetchData'](element['_latestRequestId']);

      expect(fetchCount).to.be.greaterThan(0);
      const countBefore = fetchCount;
      expect(fetchTraceIdsArg.length).to.equal(10);

      element['_showAllTraces'] = true;
      await element.updateComplete;

      await element['_fetchData'](element['_latestRequestId']);

      expect(fetchCount).to.be.greaterThan(countBefore);
      expect(fetchTraceIdsArg.length).to.equal(20);
    } finally {
      (DataService as any).instance = oldInstance;
      TraceDatabase.prototype.get = oldGet;
      TraceDatabase.prototype.set = oldSet;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => oldSubtle,
        configurable: true,
      });
    }
  });

  it('loads worker via Blob URL when fetch succeeds', async () => {
    const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;

    const mockWorker = {
      postMessage: () => {},
      terminate: () => {},
    };

    let workerCreatedWithUrl = '';
    const originalWorker = window.Worker;
    (window as any).Worker = function (url: string) {
      workerCreatedWithUrl = url;
      return mockWorker;
    } as any;

    const originalWorkerUrl = (window as any).WORKER_URL;
    delete (window as any).WORKER_URL; // Use default path (/dist/explore-multi-v2-sk/filter.worker.js)

    const originalFetch = window.fetch;
    window.fetch = async () =>
      ({
        ok: true,
        text: async () => 'console.log("mock worker code")',
      }) as any;

    try {
      document.body.appendChild(newElement);
      // Wait a bit for the fetch promise to resolve and worker to be created
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(workerCreatedWithUrl).to.match(/^blob:/);
    } finally {
      window.Worker = originalWorker;
      window.fetch = originalFetch;
      (window as any).WORKER_URL = originalWorkerUrl;
      document.body.removeChild(newElement);
    }
  });

  it('applies default param selections on load if queries empty', async () => {
    const mockDataService = {
      getInitPage: async () => ({ dataframe: { paramset: {} } }),
      getDefaults: async () => ({
        default_param_selections: { branch_name: ['aosp-androidx-main'] },
      }),
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService as any;

    try {
      const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;
      document.body.appendChild(newElement);
      await newElement.updateComplete;

      // Wait for _fetchMetadata to complete!
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect((newElement as any).queries.length).to.equal(1);
      expect((newElement as any).queries[0]['branch_name']).to.deep.equal(['aosp-androidx-main']);

      document.body.removeChild(newElement);
    } finally {
      (DataService as any).instance = oldInstance;
    }
  });

  it('applies default_xaxis_domain and default_url_values on load', async () => {
    const mockDataService = {
      getInitPage: async () => ({ dataframe: { paramset: {} } }),
      getDefaults: async () => ({
        default_xaxis_domain: 'date',
        default_url_values: {
          evenXAxisSpacing: 'true',
          sparklines: 'true',
          showZero: 'true',
        },
      }),
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService as any;

    try {
      const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;
      document.body.appendChild(newElement);
      await newElement.updateComplete;

      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(newElement.dateMode).to.be.true;
      expect((newElement as any)._evenXAxisSpacing).to.be.true;
      expect((newElement as any)._showSparklines).to.be.true;
      expect((newElement as any)._showZero).to.be.true;

      document.body.removeChild(newElement);
    } finally {
      (DataService as any).instance = oldInstance;
    }
  });

  it('has sophisticated tour steps', () => {
    expect((element as any)._tourSteps.length).to.equal(11);
    expect((element as any)._tourSteps[0].title).to.equal('Dynamic Setup');
  });

  it('applies conditional defaults on selection', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['timeNs'] },
        apply: [{ param: 'stat', values: ['min'], select_only_first: true }],
      },
    ];
    (element as any).queries = [{ metric: [] }];

    (element as any)._handleSetSelected(
      0,
      new CustomEvent('set-selected', {
        detail: { key: 'metric', values: ['timeNs'] },
      })
    );

    expect((element as any).queries[0]['stat']).to.deep.equal(['min']);
  });

  it('applies conditional defaults on add query', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['timeNs'] },
        apply: [{ param: 'stat', values: ['min'], select_only_first: true }],
      },
    ];
    (element as any).queries = [{ metric: [] }];

    (element as any)._handleAddQuery(
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'timeNs' },
      })
    );

    expect((element as any).queries[0]['stat']).to.deep.equal(['min']);
  });

  it('applies multiple values when select_only_first is false', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['frameDurationCpuMs'] },
        apply: [{ param: 'stat', values: ['P50', 'P90'], select_only_first: false }],
      },
    ];
    (element as any).queries = [{ metric: [] }];

    (element as any)._handleAddQuery(
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'frameDurationCpuMs' },
      })
    );

    expect((element as any).queries[0]['stat']).to.deep.equal(['P50', 'P90']);
  });

  it('applies only first value when select_only_first is true', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['frameDurationCpuMs'] },
        apply: [{ param: 'stat', values: ['P50', 'P90'], select_only_first: true }],
      },
    ];
    (element as any).queries = [{ metric: [] }];

    (element as any)._handleAddQuery(
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'frameDurationCpuMs' },
      })
    );

    expect((element as any).queries[0]['stat']).to.deep.equal(['P50']);
  });

  it('does not apply conditional defaults when trigger does not match', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['timeNs'] },
        apply: [{ param: 'stat', values: ['min'], select_only_first: true }],
      },
    ];
    (element as any).queries = [{ metric: [] }];

    (element as any)._handleAddQuery(
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'otherMetric' },
      })
    );

    expect((element as any).queries[0]['stat']).to.be.undefined;
  });

  it('does not re-apply conditional defaults if the trigger value was already present', async () => {
    (element as any)._conditionalDefaults = [
      {
        trigger: { param: 'metric', values: ['timeNs'] },
        apply: [{ param: 'stat', values: ['min'], select_only_first: true }],
      },
    ];
    (element as any).queries = [{ metric: ['timeNs'], stat: ['max'] }];

    (element as any)._handleAddQuery(
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'otherMetric' },
      })
    );

    expect((element as any).queries[0]['stat']).to.deep.equal(['max']);
  });

  it('clears suggestions list on query modification events', () => {
    element['_suggestionsForQueryBar'] = [[{ params: [], score: 100 }]];

    element['_handleAddQuery'](
      0,
      new CustomEvent('add-query', {
        detail: { key: 'metric', value: 'time' },
      })
    );
    expect(element['_suggestionsForQueryBar'][0]).to.deep.equal([]);

    element['_suggestionsForQueryBar'] = [[{ params: [], score: 100 }]];
    element['_handleRemoveQuery'](
      0,
      new CustomEvent('remove-query', {
        detail: { key: 'metric', value: 'time' },
      })
    );
    expect(element['_suggestionsForQueryBar'][0]).to.deep.equal([]);

    element['_suggestionsForQueryBar'] = [[{ params: [], score: 100 }]];
    element['_handleSetSelected'](
      0,
      new CustomEvent('set-selected', {
        detail: { key: 'metric', values: ['time'] },
      })
    );
    expect(element['_suggestionsForQueryBar'][0]).to.deep.equal([]);

    element['_suggestionsForQueryBar'] = [[{ params: [], score: 100 }]];
    element['_handleRemoveKey'](
      0,
      new CustomEvent('remove-key', {
        detail: { key: 'metric' },
      })
    );
    expect(element['_suggestionsForQueryBar'][0]).to.deep.equal([]);
  });

  it('merges anomalymap from fetchTraceValues response correctly under the uncollapsed trace ID', async () => {
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (_arg: any) => {
        return {
          results: {
            ',benchmark=A,test=B,stat=value,': [{ commit_number: 50, createdat: 0, val: 1.5 }],
          },
          anomalymap: {
            ',benchmark=A,test=B,stat=value,': {
              100: {
                id: 'a1',
                is_improvement: true,
                state: 'untriaged',
                bug_id: 0,
                recovered: false,
                median_before_anomaly: 5.0,
                median_after_anomaly: 10.0,
              },
            },
          },
        };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    const oldGet = TraceDatabase.prototype.get;
    const oldSet = TraceDatabase.prototype.set;
    TraceDatabase.prototype.get = async () => null; // Force cache miss
    TraceDatabase.prototype.set = async () => {};

    const oldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    try {
      element['_matchingTraceIds'] = [',benchmark=A,test=B,stat=value,'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [
        {
          id: ',benchmark=A,test=B,stat=value,',
          rows: [{ commit_number: 150, createdat: 0, val: 1.0 }],
          color: '#fff',
        },
      ];
      element['_loadedBounds'] = { ',benchmark=A,test=B,stat=value,': { min: 100, max: 200 } };
      element['_globalBounds'] = {};

      // Trigger viewport change that requires left fetch
      await element['_doHandleViewportChanged']({
        detail: { minCommit: 50, maxCommit: 150 },
      });

      expect(element['_regressions']).to.not.be.empty;
      const reg = element['_regressions'][',benchmark=A,test=B,stat=value,'];
      expect(reg).to.not.be.undefined;
      expect(reg[100]).to.not.be.undefined;
      expect(reg[100].is_improvement).to.be.true;
      expect((reg[100] as any).status).to.equal('untriaged');
      expect((reg[100] as any).median_before).to.equal(5.0);
      expect((reg[100] as any).median_after).to.equal(10.0);
    } finally {
      (DataService as any).instance = oldInstance;
      TraceDatabase.prototype.get = oldGet;
      TraceDatabase.prototype.set = oldSet;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => oldSubtle,
        configurable: true,
      });
    }
  });

  it('uses cache in _doHandleViewportChanged and skips fetch', async () => {
    let fetchCalled = false;
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (_arg: any) => {
        fetchCalled = true;
        return { results: {} };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    const oldGet = TraceDatabase.prototype.get;
    const oldSet = TraceDatabase.prototype.set;

    const cachedData = {
      results: {
        ',benchmark=A,test=B,': [{ commit_number: 50, createdat: 0, val: 1.5 }],
      },
    };

    TraceDatabase.prototype.get = async () => cachedData;
    TraceDatabase.prototype.set = async () => {};

    const oldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    try {
      element['_matchingTraceIds'] = [',benchmark=A,test=B,'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [{ id: ',benchmark=A,test=B,', rows: [], color: '#fff' }];
      element['_loadedBounds'] = { ',benchmark=A,test=B,': { min: 100, max: 200 } };
      element['_globalBounds'] = {};

      // Trigger viewport change that requires left fetch
      await element['_doHandleViewportChanged']({
        detail: { minCommit: 50, maxCommit: 150 },
      });

      expect(fetchCalled).to.be.false; // Verified fetch skipped!

      // Verify data was merged from cache
      const series = element['_seriesData'][0];
      expect(series.rows.length).to.equal(1);
      expect(series.rows[0].commit_number).to.equal(50);
      expect(series.rows[0].val).to.equal(1.5);
    } finally {
      (DataService as any).instance = oldInstance;
      TraceDatabase.prototype.get = oldGet;
      TraceDatabase.prototype.set = oldSet;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => oldSubtle,
        configurable: true,
      });
    }
  });

  it('merges stat=median rows cleanly as independent series', async () => {
    const mockDataService = {
      getLinksBatch: async () => ({}),
      fetchTraceValues: async (_arg: any) => {
        return {
          results: {
            ',benchmark=A,test=B,stat=median,': [{ commit_number: 50, createdat: 0, val: 1.5 }],
          },
        };
      },
    };
    const oldInstance = (DataService as any).instance;
    (DataService as any).instance = mockDataService;

    const oldGet = TraceDatabase.prototype.get;
    const oldSet = TraceDatabase.prototype.set;
    TraceDatabase.prototype.get = async () => null;
    TraceDatabase.prototype.set = async () => {};

    const oldSubtle = window.crypto.subtle;
    Object.defineProperty(window.crypto, 'subtle', {
      get: () => ({
        digest: async () => new ArrayBuffer(32),
      }),
      configurable: true,
    });

    try {
      element['_matchingTraceIds'] = [',benchmark=A,test=B,stat=median,'];
      element['_tracePage'] = 0;
      element['_pageSize'] = 10;
      element['_seriesData'] = [
        { id: ',benchmark=A,test=B,stat=median,', rows: [], color: '#fff' },
      ];
      element['_loadedBounds'] = { ',benchmark=A,test=B,stat=median,': { min: 100, max: 200 } };
      element['_globalBounds'] = {};

      element['_defaultParamSelections'] = { stat: ['median'] };

      await element['_doHandleViewportChanged']({
        detail: { minCommit: 50, maxCommit: 150 },
      });

      const series = element['_seriesData'][0];
      expect(series.id).to.equal(',benchmark=A,test=B,stat=median,');
      expect(series.rows.length).to.equal(1);
      expect(series.rows[0].val).to.equal(1.5);
    } finally {
      (DataService as any).instance = oldInstance;
      TraceDatabase.prototype.get = oldGet;
      TraceDatabase.prototype.set = oldSet;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => oldSubtle,
        configurable: true,
      });
    }
  });

  it('renders config pills when diffBase or splitKeys is set', async () => {
    element['_diffBase'] = { key: 'test', value: 'Score' };
    element['splitKeys'] = new Set(['bot']);
    await element.updateComplete;

    const pills = element.shadowRoot!.querySelectorAll('.config-pill');
    expect(pills.length).to.equal(2);

    const diffPill = Array.from(pills).find((p) => p.textContent?.includes('Diff Base:'));
    expect(diffPill).to.not.be.undefined;
    expect(diffPill!.textContent).to.include('Score');

    const splitPill = Array.from(pills).find((p) => p.textContent?.includes('Split by:'));
    expect(splitPill).to.not.be.undefined;
    expect(splitPill!.textContent).to.include('bot');
  });

  it('adds and removes split keys when split event is fired', async () => {
    expect(element['splitKeys'].has('bot')).to.be.false;

    const toolbar = element.shadowRoot!.querySelector('explore-toolbar-sk');
    expect(toolbar).to.not.be.null;

    // Fire split event to add 'bot'
    toolbar!.dispatchEvent(
      new CustomEvent('split', { detail: { key: 'bot' }, bubbles: true, composed: true })
    );
    await element.updateComplete;

    expect(element['splitKeys'].has('bot')).to.be.true;

    // Fire split event to remove 'bot'
    toolbar!.dispatchEvent(
      new CustomEvent('split', { detail: { key: 'bot' }, bubbles: true, composed: true })
    );
    await element.updateComplete;

    expect(element['splitKeys'].has('bot')).to.be.false;
  });

  it('removes split keys when clicking remove button on the config pill', async () => {
    element['splitKeys'] = new Set(['bot']);
    await element.updateComplete;

    const splitPill = Array.from(element.shadowRoot!.querySelectorAll('.config-pill')).find((p) =>
      p.textContent?.includes('Split by:')
    );
    expect(splitPill).to.not.be.undefined;

    const removeBtn = splitPill!.querySelector('.config-pill-remove') as HTMLButtonElement;
    expect(removeBtn).to.not.be.null;

    removeBtn.click();
    await element.updateComplete;

    expect(element['splitKeys'].has('bot')).to.be.false;
  });

  it('loads queries from a shortcut ID', async () => {
    const mockShortcutConfigs = [
      { queries: ['test=A&stat=value'], formulas: [], keys: '' },
      { queries: ['benchmark=B'], formulas: [], keys: '' },
    ];
    let getShortcutId = '';
    const originalGetShortcut = DataService.prototype.getShortcut;
    DataService.prototype.getShortcut = async (id: string) => {
      getShortcutId = id;
      return mockShortcutConfigs as any;
    };

    try {
      await element['_loadShortcut']('mock-shortcut-123');

      expect(getShortcutId).to.equal('mock-shortcut-123');
      expect(element['queries'].length).to.equal(2);
      expect(element['queries'][0]).to.deep.equal({ test: ['A'], stat: ['value'] });
      expect(element['queries'][1]).to.deep.equal({ benchmark: ['B'] });
      expect(element['_lastLoadedShortcut']).to.equal('mock-shortcut-123');
    } finally {
      DataService.prototype.getShortcut = originalGetShortcut;
    }
  });

  it('updates the shortcut ID in state and URL when queries change', async () => {
    const originalUpdateShortcut = DataService.prototype.updateShortcut;
    let updateConfigs: any = null;
    DataService.prototype.updateShortcut = async (configs: any) => {
      updateConfigs = configs;
      return 'new-shortcut-id-456';
    };

    try {
      element['queries'] = [{ test: ['X'] }];
      await element['_updateShortcut']();

      expect(updateConfigs).to.not.be.null;
      expect(updateConfigs.length).to.equal(1);
      expect(updateConfigs[0].queries).to.deep.equal(['test=X']);
      expect(element['_shortcut']).to.equal('new-shortcut-id-456');

      const url = new URL(window.location.href);
      expect(url.searchParams.get('shortcut')).to.equal('new-shortcut-id-456');
    } finally {
      DataService.prototype.updateShortcut = originalUpdateShortcut;
    }
  });

  it('clears shortcut ID from state and URL when queries are emptied', async () => {
    const originalUpdateShortcut = DataService.prototype.updateShortcut;
    let updateCalled = false;
    DataService.prototype.updateShortcut = async () => {
      updateCalled = true;
      return 'test-id';
    };

    try {
      // Start with a valid query and shortcut
      element['queries'] = [{ test: ['Z'] }];
      element['_shortcut'] = 'some-existing-id';
      element['_lastQueriesJson'] = JSON.stringify([{ test: ['Z'] }]);

      // Now clear the query
      element['queries'] = [{}];
      await element['_updateShortcut']();

      expect(updateCalled).to.be.false; // Should not call updateShortcut when empty
      expect(element['_shortcut']).to.equal('');

      const url = new URL(window.location.href);
      expect(url.searchParams.get('shortcut')).to.be.null;
    } finally {
      DataService.prototype.updateShortcut = originalUpdateShortcut;
    }
  });

  it('handles network errors when loading a shortcut gracefully', async () => {
    const originalGetShortcut = DataService.prototype.getShortcut;
    DataService.prototype.getShortcut = async () => {
      throw new Error('Network failure');
    };

    try {
      element['queries'] = [{ existing: ['true'] }];
      element['_lastLoadedShortcut'] = '';

      await element['_loadShortcut']('bad-shortcut-id');

      // Queries should remain unchanged from the existing queries
      expect(element['queries']).to.deep.equal([{ existing: ['true'] }]);
    } finally {
      DataService.prototype.getShortcut = originalGetShortcut;
    }
  });

  it('populates optionsByKeyPerQuery[0] with overridden options for active facets', async () => {
    element['queries'] = [{ benchmark: ['v8'] }];
    element['_latestActiveFacets'] = ['benchmark'];

    const payload = {
      queryResults: [
        { paramsByKey: { benchmark: [{ value: 'v8', count: 10 }] } }, // result for full query
        {
          paramsByKey: {
            benchmark: [
              { value: 'v8', count: 10 },
              { value: 'v8.infra', count: 5 },
            ],
          },
        }, // result for query with benchmark removed
      ],
    };

    element['_handleFilterResult'](payload);

    expect(element['_optionsByKeyPerQuery'][0]['benchmark']).to.deep.equal([
      { value: 'v8', count: 10 },
      { value: 'v8.infra', count: 5 },
    ]);
  });

  describe('expand/collapse query bars', () => {
    it('collapses query bars to first 3 by default when there are more than 3', async () => {
      element.queries = [{}, {}, {}, {}, {}];
      await element.updateComplete;

      const queryBars = element.shadowRoot!.querySelectorAll('query-bar-sk');
      expect(queryBars.length).to.equal(3);

      const expandBtn = element.shadowRoot!.querySelector(
        '.expand-queries-btn'
      ) as HTMLButtonElement;
      expect(expandBtn).to.not.be.null;
      expect(expandBtn.textContent?.trim()).to.equal('Expand (2 more)');
    });

    it('expands query bars and changes label when expand button clicked', async () => {
      element.queries = [{}, {}, {}, {}, {}];
      await element.updateComplete;

      const expandBtn = element.shadowRoot!.querySelector(
        '.expand-queries-btn'
      ) as HTMLButtonElement;
      expandBtn.click();
      await element.updateComplete;

      const queryBars = element.shadowRoot!.querySelectorAll('query-bar-sk');
      expect(queryBars.length).to.equal(5);
      expect(expandBtn.textContent?.trim()).to.equal('Collapse');
    });

    it('collapses query bars back to 3 when collapse button clicked', async () => {
      element.queries = [{}, {}, {}, {}, {}];
      (element as any)._queriesExpanded = true;
      await element.updateComplete;

      const expandBtn = element.shadowRoot!.querySelector(
        '.expand-queries-btn'
      ) as HTMLButtonElement;
      expect(expandBtn.textContent?.trim()).to.equal('Collapse');

      expandBtn.click();
      await element.updateComplete;

      const queryBars = element.shadowRoot!.querySelectorAll('query-bar-sk');
      expect(queryBars.length).to.equal(3);
      expect(expandBtn.textContent?.trim()).to.equal('Expand (2 more)');
    });

    it('auto-expands query bars when add query button is clicked and total becomes > 3', async () => {
      element.queries = [{}, {}, {}];
      await element.updateComplete;

      const addBtn = element.shadowRoot!.querySelector(
        '.add-query-circle-btn'
      ) as HTMLButtonElement;
      addBtn.click();
      await element.updateComplete;

      expect((element as any)._queriesExpanded).to.be.true;
      const queryBars = element.shadowRoot!.querySelectorAll('query-bar-sk');
      expect(queryBars.length).to.equal(4);
    });
  });

  describe('cloning query bars', () => {
    it('duplicates query parameters and inserts the new query next to the cloned one', async () => {
      element.queries = [{ benchmark: ['blink_perf'] }, { device: ['m1'] }];
      await element.updateComplete;

      // Simulate clone-query event on the first query bar
      const firstQueryBar = element.shadowRoot!.querySelectorAll('query-bar-sk')[0];
      expect(firstQueryBar).to.not.be.undefined;

      firstQueryBar.dispatchEvent(new CustomEvent('clone-query', { bubbles: true }));
      await element.updateComplete;

      expect(element.queries.length).to.equal(3);
      expect(element.queries[0]).to.deep.equal({ benchmark: ['blink_perf'] });
      expect(element.queries[1]).to.deep.equal({ benchmark: ['blink_perf'] }); // inserted copy next to it
      expect(element.queries[2]).to.deep.equal({ device: ['m1'] });
    });

    it('auto-expands query bars if total count becomes > 3 after cloning', async () => {
      element.queries = [{ benchmark: ['blink_perf'] }, { device: ['m1'] }, { arch: ['x86'] }];
      (element as any)._queriesExpanded = false;
      await element.updateComplete;

      const firstQueryBar = element.shadowRoot!.querySelectorAll('query-bar-sk')[0];
      firstQueryBar.dispatchEvent(new CustomEvent('clone-query', { bubbles: true }));
      await element.updateComplete;

      expect((element as any)._queriesExpanded).to.be.true;
      const queryBars = element.shadowRoot!.querySelectorAll('query-bar-sk');
      expect(queryBars.length).to.equal(4);
    });
  });

  describe('V2 Toggle', () => {
    beforeEach(() => {
      localStorage.removeItem('perf:use-explore-v2');
    });

    afterEach(() => {
      localStorage.removeItem('perf:use-explore-v2');
    });

    it('_toggleV2Mode should save preference and redirect to /m', async () => {
      const redirectStub = sinon.stub(element, 'redirect');

      await element['_toggleV2Mode']();

      expect(localStorage.getItem('perf:use-explore-v2')).to.equal('false');
      expect(redirectStub.calledOnce).to.be.true;
      expect(redirectStub.firstCall.args[0]).to.include('/m');
    });
  });

  describe('request ID tracking (race conditions)', () => {
    it('discards out-of-order fetch results', async () => {
      let resolveFetch1: (value: any) => void = () => {};
      let resolveFetch2: (value: any) => void = () => {};

      const fetch1Promise = new Promise((resolve) => {
        resolveFetch1 = resolve;
      });
      const fetch2Promise = new Promise((resolve) => {
        resolveFetch2 = resolve;
      });

      let resolveFetch1Started: (value: any) => void = () => {};
      let resolveFetch2Started: (value: any) => void = () => {};
      const fetch1StartedPromise = new Promise((resolve) => {
        resolveFetch1Started = resolve;
      });
      const fetch2StartedPromise = new Promise((resolve) => {
        resolveFetch2Started = resolve;
      });

      const mockDataService = {
        getLinksBatch: async () => ({}),
        sendFrameRequest: async (req: any) => {
          if (req.trace_ids.includes('t1')) {
            resolveFetch1Started(null);
            await fetch1Promise;
            return {
              dataframe: {
                header: [{ offset: 10, timestamp: 1000 }],
                traceset: { t1: [1.0] },
              },
            };
          } else if (req.trace_ids.includes('t2')) {
            resolveFetch2Started(null);
            await fetch2Promise;
            return {
              dataframe: {
                header: [{ offset: 10, timestamp: 1000 }],
                traceset: { t2: [2.0] },
              },
            };
          }
          throw new Error('Unexpected request: ' + JSON.stringify(req));
        },
      };

      const oldInstance = (DataService as any).instance;
      (DataService as any).instance = mockDataService;

      const oldGet = TraceDatabase.prototype.get;
      const oldSet = TraceDatabase.prototype.set;
      TraceDatabase.prototype.get = async () => null;
      TraceDatabase.prototype.set = async () => {};

      const oldSubtle = window.crypto.subtle;
      Object.defineProperty(window.crypto, 'subtle', {
        get: () => ({
          digest: async () => new ArrayBuffer(32),
        }),
        configurable: true,
      });

      try {
        element['_tracePage'] = 0;
        element['_pageSize'] = 10;
        element['_seriesData'] = [];

        // Start Fetch 1 (requestId = 1)
        element['_matchingTraceIds'] = ['t1'];
        element['_latestRequestId'] = 1;
        const p1 = element['_fetchData'](1);

        // Wait for Fetch 1 to reach sendFrameRequest
        await fetch1StartedPromise;

        // Start Fetch 2 (requestId = 2)
        element['_matchingTraceIds'] = ['t2'];
        element['_latestRequestId'] = 2;
        const p2 = element['_fetchData'](2);

        // Wait for Fetch 2 to reach sendFrameRequest
        await fetch2StartedPromise;

        // Resolve Fetch 2 first
        resolveFetch2(null);
        await p2;

        // Verify Fetch 2 data is applied
        expect(element['_seriesData'].map((s: any) => s.id)).to.deep.equal(['t2']);

        // Resolve Fetch 1 later
        resolveFetch1(null);
        await p1;

        // Verify Fetch 1 data is DISCARDED (we still have Fetch 2 data)
        expect(element['_seriesData'].map((s: any) => s.id)).to.deep.equal(['t2']);
      } finally {
        (DataService as any).instance = oldInstance;
        TraceDatabase.prototype.get = oldGet;
        TraceDatabase.prototype.set = oldSet;
        Object.defineProperty(window.crypto, 'subtle', {
          get: () => oldSubtle,
          configurable: true,
        });
      }
    });
  });

  describe('trace color assignment', () => {
    it('assigns unique and stable colors based on position in _seriesData', async () => {
      // Set seriesData using the helper method
      (element as any)['_updateSeriesData']([
        { id: 't1', rows: [], color: 'red' },
        { id: 't2', rows: [], color: 'red' },
      ]);

      // They should have been reassigned unique colors
      const c1 = element['_seriesData'][0].color;
      const c2 = element['_seriesData'][1].color;
      expect(c1).to.not.equal(c2);
      expect(c1).to.equal('hsl(0, 70%, 50%)');
      expect(c2).to.equal('hsl(137.5, 70%, 50%)');

      // Now add a third series
      (element as any)['_updateSeriesData']([
        ...element['_seriesData'],
        { id: 't3', rows: [], color: 'red' },
      ]);

      const c3 = element['_seriesData'][2].color;
      expect(element['_seriesData'][0].color).to.equal(c1); // stable
      expect(element['_seriesData'][1].color).to.equal(c2); // stable
      expect(c3).to.equal('hsl(275, 70%, 50%)'); // unique
    });
  });

  it('shows loading overlay during worker initialization and hides it when ready', async () => {
    const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;

    const WORKER_LOADED_DELAY = 5;
    const WORKER_STATUS_DELAY = 10;
    const WORKER_READY_DELAY = 20;

    const STEP1_WAIT = 2; // Before WORKER_LOADED_DELAY
    const STEP2_WAIT = 15; // After WORKER_LOADED_DELAY + WORKER_STATUS_DELAY (5 + 10 = 15)
    const STEP3_WAIT = 15; // Wait another 15ms (total 32ms), ensuring we are past the READY event (at 25ms)

    let workerOnMessage: ((e: MessageEvent) => void) | null = null;
    const mockWorker = {
      postMessage: (msg: any) => {
        if (msg.type === 'INIT') {
          setTimeout(() => {
            workerOnMessage!({
              data: { type: 'STATUS', payload: { message: 'Loading database...' } },
            } as any);
          }, WORKER_STATUS_DELAY);
          setTimeout(() => {
            workerOnMessage!({ data: { type: 'READY' } } as any);
          }, WORKER_READY_DELAY);
        }
      },
      terminate: () => {},
    };

    const originalWorker = window.Worker;
    (window as any).Worker = function (_: string) {
      const w = Object.create(mockWorker);
      Object.defineProperty(w, 'onmessage', {
        set: (handler) => {
          workerOnMessage = handler;
        },
        get: () => workerOnMessage,
      });
      setTimeout(() => {
        workerOnMessage!({ data: { type: 'LOADED' } } as any);
      }, WORKER_LOADED_DELAY);
      return w;
    } as any;

    try {
      document.body.appendChild(newElement);

      // 1. Initial connection status
      await new Promise((resolve) => setTimeout(resolve, STEP1_WAIT));
      let overlay = newElement.shadowRoot!.querySelector('.worker-init-overlay');
      let status = newElement.shadowRoot!.querySelector('.worker-init-status');
      expect(overlay).to.not.be.null;
      expect(status!.textContent).to.equal('Starting worker...');

      // 2. Updated status from worker
      await new Promise((resolve) => setTimeout(resolve, STEP2_WAIT));
      status = newElement.shadowRoot!.querySelector('.worker-init-status');
      expect(status!.textContent).to.equal('Loading database...');

      // 3. Hidden when READY
      await new Promise((resolve) => setTimeout(resolve, STEP3_WAIT));
      overlay = newElement.shadowRoot!.querySelector('.worker-init-overlay');
      expect(overlay).to.be.null;
    } finally {
      window.Worker = originalWorker;
      newElement.parentNode?.removeChild(newElement);
    }
  });

  it('dismisses loading overlay on worker initialization failure', async () => {
    const newElement = document.createElement('explore-multi-v2-sk') as ExploreMultiV2Sk;

    let workerOnMessage: ((e: MessageEvent) => void) | null = null;
    const mockWorker = {
      postMessage: (msg: any) => {
        if (msg.type === 'INIT') {
          setTimeout(() => {
            workerOnMessage!({
              data: { type: 'ERROR', payload: { message: 'Failed to fetch Wasm' } },
            } as any);
          }, 10);
        }
      },
      terminate: () => {},
    };

    const originalWorker = window.Worker;
    (window as any).Worker = function (_: string) {
      const w = Object.create(mockWorker);
      Object.defineProperty(w, 'onmessage', {
        set: (handler) => {
          workerOnMessage = handler;
        },
        get: () => workerOnMessage,
      });
      setTimeout(() => {
        workerOnMessage!({ data: { type: 'LOADED' } } as any);
      }, 5);
      return w;
    } as any;

    try {
      document.body.appendChild(newElement);

      // Poll until overlay is dismissed (should happen after ERROR)
      const startTime = Date.now();
      while (
        newElement.shadowRoot!.querySelector('.worker-init-overlay') &&
        Date.now() - startTime < 2000
      ) {
        await new Promise((resolve) => setTimeout(resolve, 10));
        await newElement.updateComplete;
      }

      const overlay = newElement.shadowRoot!.querySelector('.worker-init-overlay');
      expect(overlay).to.be.null;
    } finally {
      window.Worker = originalWorker;
      newElement.parentNode?.removeChild(newElement);
    }
  });

  describe('reset zoom', () => {
    it('does not clear series data or trigger _fetchData', async () => {
      let fetchDataCalled = false;
      (element as any)._fetchData = async () => {
        fetchDataCalled = true;
      };
      (element as any)._seriesData = [{ id: 'trace-1', rows: [] }];

      (element as any)._onResetZoom();

      expect(fetchDataCalled).to.be.false;
      expect((element as any)._seriesData.length).to.equal(1);
    });

    it('resets viewportMinX and viewportMaxX to null on standard explore page', () => {
      element.embedded = false;
      element.viewportMinX = 100;
      element.viewportMaxX = 500;

      (element as any)._onResetZoom();

      expect(element.viewportMinX).to.be.null;
      expect(element.viewportMaxX).to.be.null;
    });

    it('resets viewport to anomaly commit range [minCommit - 100, maxCommit + 100] when embedded = true', () => {
      element.embedded = true;
      (element as any)._regressions = {
        'trace-1': {
          500: {
            id: 'anomaly-1',
            start_revision: 450,
            end_revision: 500,
            commit_number: 500,
          },
          800: {
            id: 'anomaly-2',
            start_revision: 750,
            end_revision: 800,
            commit_number: 800,
          },
        },
      };

      (element as any)._onResetZoom();

      // minCommit = 450, maxCommit = 800
      // Expected range: [max(0, 450 - 100), 800 + 100] = [350, 900]
      expect(element.viewportMinX).to.equal(350);
      expect(element.viewportMaxX).to.equal(900);
    });

    it('filters anomaly range by highlightAnomalies when set on embedded page', () => {
      element.embedded = true;
      element.highlightAnomalies = ['anomaly-2'];
      (element as any)._regressions = {
        'trace-1': {
          500: {
            id: 'anomaly-1',
            start_revision: 450,
            end_revision: 500,
            commit_number: 500,
          },
          800: {
            id: 'anomaly-2',
            start_revision: 750,
            end_revision: 800,
            commit_number: 800,
          },
        },
      };

      (element as any)._onResetZoom();

      // Only anomaly-2 is highlighted: minCommit = 750, maxCommit = 800
      // Expected range: [750 - 100, 800 + 100] = [650, 900]
      expect(element.viewportMinX).to.equal(650);
      expect(element.viewportMaxX).to.equal(900);
    });
  });
});
