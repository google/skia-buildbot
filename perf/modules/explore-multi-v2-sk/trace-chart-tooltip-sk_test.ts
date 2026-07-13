import './trace-chart-tooltip-sk';
import { TraceChartTooltipSk } from './trace-chart-tooltip-sk';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';

describe('trace-chart-tooltip-sk', () => {
  let element: TraceChartTooltipSk;

  beforeEach(async () => {
    if (!(window as any).perf) {
      (window as any).perf = {};
    }
    (window as any).perf.commit_range_url = '';
    fetchMock.post('/_/cid/', { commitSlice: [] });
    fetchMock.post('/_/details/?results=false', { cid: '', anomalies: [] });
    fetchMock.post('/_/links/', {});
    element = document.createElement('trace-chart-tooltip-sk') as TraceChartTooltipSk;
    element.bug_host_url = 'https://issues.chromium.org';
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
    fetchMock.reset();
  });

  it('renders anomaly details when available', async () => {
    const rows = [
      { commit_number: 99, val: 9.0, createdat: 500, hash: 'hash99' },
      { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
    ];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[1],
      x: 100,
      y: 100,
    };
    element.regressions = {
      test: {
        100: {
          id: '123',
          bugs: [{ bug_id: '456', bug_type: 'BUGANIZER' }],
          is_improvement: false,
          median_before: 5.0,
          median_after: 10.0,
        } as any,
      },
    };
    await element.updateComplete;

    const anomalyText = element.shadowRoot!.textContent;
    expect(anomalyText).to.include('Regression');
    expect(anomalyText).to.include('Bug ID');
  });

  it('renders both date and commit number', async () => {
    const rows = [{ commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' }];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[0],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const text = element.shadowRoot!.textContent;
    expect(text).to.include('Date:');
    expect(text).to.include('Commit Number:');
  });

  it('renders commit-range-sk in tooltip', async () => {
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const commitRange = element.shadowRoot!.querySelector('commit-range-sk');
    expect(commitRange).to.not.be.null;
  });

  it('renders json-source-sk when enabled', async () => {
    if (!(window as any).perf) {
      (window as any).perf = {};
    }
    const oldShowJson = (window as any).perf.show_json_file_display;
    (window as any).perf.show_json_file_display = true;
    try {
      element.hoveredPoint = {
        series: { id: 'test', color: '#fff', rows: [] },
        row: { commit_number: 100, val: 10.0, createdat: 1000 },
        x: 100,
        y: 100,
      };
      await element.updateComplete;

      const jsonSource = element.shadowRoot!.querySelector('json-source-sk');
      expect(jsonSource).to.not.be.null;
    } finally {
      (window as any).perf.show_json_file_display = oldShowJson;
    }
  });

  it('sets properties on json-source-sk', async () => {
    if (!(window as any).perf) {
      (window as any).perf = {};
    }
    const oldShowJson = (window as any).perf.show_json_file_display;
    (window as any).perf.show_json_file_display = true;
    try {
      element.hoveredPoint = {
        series: { id: 'test', color: '#fff', rows: [] },
        row: { commit_number: 100, val: 10.0, createdat: 1000 },
        x: 100,
        y: 100,
      };
      await element.updateComplete;

      const jsonSource = element.shadowRoot!.querySelector('json-source-sk') as any;
      expect(jsonSource).to.not.be.null;
      expect(jsonSource.cid).to.equal(100);
      expect(jsonSource.traceid).to.equal('test');
    } finally {
      (window as any).perf.show_json_file_display = oldShowJson;
    }
  });

  it('renders user-issue-sk in tooltip when no regression', async () => {
    const rows = [
      { commit_number: 99, val: 9.0, createdat: 500, hash: 'hash99' },
      { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
    ];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[1],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const userIssue = element.shadowRoot!.querySelector('user-issue-sk');
    expect(userIssue).to.not.be.null;
  });

  it('renders try-job button when enabled', async () => {
    const rows = [
      { commit_number: 99, val: 9.0, createdat: 500, hash: 'hash99' },
      { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
    ];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[1],
      x: 100,
      y: 100,
    };
    element.showPinpointButtons = true;
    await element.updateComplete;

    const tryJobBtn = element.shadowRoot!.querySelector('#try-job');
    expect(tryJobBtn).to.not.be.null;
  });

  it('pre-fills bisect dialog params correctly', async () => {
    element.processedSeries = [
      {
        id: ',master=Chromium,bot=linux,benchmark=blink_perf,test=layout,subtest_1=story1,',
        color: '#fff',
        rows: [
          { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
          { commit_number: 101, val: 11.0, createdat: 2000, hash: 'hash101' },
        ],
      },
    ];
    element.hoveredPoint = {
      series: element.processedSeries[0],
      row: element.processedSeries[0].rows[1],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const pinpointDialog = element.shadowRoot!.querySelector('#pinpoint-dialog-sk') as any;
    expect(pinpointDialog).to.not.be.null;

    let calledMode: string | null = null;
    let calledParams: any = null;
    pinpointDialog.open = (mode: string, params: any) => {
      calledMode = mode;
      calledParams = params;
    };

    (element as any)['openBisectDialog']();

    expect(calledMode).to.equal('bisect');
    expect(calledParams).to.not.be.null;
    expect(calledParams.testPath).to.equal('Chromium/linux/blink_perf/layout/story1');
    expect(calledParams.startCommit).to.equal('100');
    expect(calledParams.endCommit).to.equal('101');
    expect(calledParams.story).to.equal('story1');
  });

  it('pre-fills try-job dialog params correctly', async () => {
    element.processedSeries = [
      {
        id: ',master=Chromium,bot=linux,benchmark=blink_perf,test=layout,subtest_1=story1,',
        color: '#fff',
        rows: [
          { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
          { commit_number: 101, val: 11.0, createdat: 2000, hash: 'hash101' },
        ],
      },
    ];
    element.hoveredPoint = {
      series: element.processedSeries[0],
      row: element.processedSeries[0].rows[1],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const pinpointDialog = element.shadowRoot!.querySelector('#pinpoint-dialog-sk') as any;
    expect(pinpointDialog).to.not.be.null;

    let calledMode: string | null = null;
    let calledParams: any = null;
    pinpointDialog.open = (mode: string, params: any) => {
      calledMode = mode;
      calledParams = params;
    };

    (element as any)['openTryJobDialog']();

    expect(calledMode).to.equal('try');
    expect(calledParams).to.not.be.null;
    expect(calledParams.testPath).to.equal('Chromium/linux/blink_perf/layout/story1');
    expect(calledParams.baseCommit).to.equal('100');
    expect(calledParams.endCommit).to.equal('101');
    expect(calledParams.story).to.equal('story1');
  });

  it('uses hashes from rows and disables autoload', async () => {
    // Set dummy point to force rendering of template so we can query commit-range-sk
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const commitRange = element.shadowRoot!.querySelector('commit-range-sk') as any;
    expect(commitRange).to.not.be.null;
    commitRange.autoload = false;

    element.processedSeries = [
      {
        id: 'test',
        color: '#fff',
        rows: [
          { commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' },
          { commit_number: 101, val: 11.0, createdat: 2000, hash: 'hash101' },
        ],
      },
    ];
    element.hoveredPoint = {
      series: element.processedSeries[0],
      row: element.processedSeries[0].rows[1],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    expect(commitRange).to.not.be.null;

    // Wait for the debounce timeout
    await new Promise((resolve) => setTimeout(resolve, 200));
    await element.updateComplete;

    expect((commitRange as any)['_autoload']).to.be.false;
    expect(commitRange.hashes).to.deep.equal(['hash100', 'hash101']);
  });

  it('renders sections for grouping', async () => {
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const sections = element.shadowRoot!.querySelectorAll('.tooltip-section');
    expect(sections.length).to.be.greaterThan(0);
  });

  it('renders series color indicator', async () => {
    element.hoveredPoint = {
      series: { id: 'test', color: '#ff0000', rows: [] },
      row: { commit_number: 100, val: 10.0, createdat: 1000 },
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const indicator = element.shadowRoot!.querySelector('.color-indicator');
    expect(indicator).to.not.be.null;
  });

  it('uses css variables for background and color', async () => {
    const styles = (TraceChartTooltipSk as any).styles.cssText;
    expect(styles).to.include('var(--surface');
    expect(styles).to.include('var(--on-surface');
  });

  it('calculates nudge list sparse correctly', () => {
    const rows = [
      { commit_number: 100, val: 10.0, createdat: 1000 },
      { commit_number: 101, val: 11.0, createdat: 2000 },
      { commit_number: 102, val: 12.0, createdat: 3000 },
    ];
    const anomalyData = {
      anomaly: { start_revision: 100, end_revision: 101 } as any,
      x: 1,
      y: 11.0,
      highlight: false,
    };
    const nudgeList = (element as any)['calculateNudgeListSparse'](
      rows,
      1,
      anomalyData,
      1,
      0,
      true
    );

    expect(nudgeList.length).to.equal(3);
    expect(nudgeList[0].display_index).to.equal(-1);
    expect(nudgeList[0].start_revision).to.equal(100);
    expect(nudgeList[1].display_index).to.equal(0);
    expect(nudgeList[2].display_index).to.equal(1);
  });

  it('renders skeleton loaders when author/message are missing', async () => {
    const rows = [{ commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' }];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[0],
      x: 100,
      y: 100,
    };
    await element.updateComplete;

    const skeleton = element.shadowRoot!.querySelector('.skeleton');
    expect(skeleton).to.not.be.null;
  });

  it('renders subrepo skeleton when loading', async () => {
    const rows = [{ commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' }];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[0],
      x: 100,
      y: 100,
    };
    // Simulate loading state
    (element as any)._linksLoading = true;
    await element.updateComplete;

    const subrepoSkeleton = element.shadowRoot!.querySelector('.subrepo-skeleton');
    expect(subrepoSkeleton).to.not.be.null;
  });

  it('stops propagation of pointer and mouse events', async () => {
    const events = [
      'pointerdown',
      'pointermove',
      'pointerup',
      'click',
      'dblclick',
      'wheel',
      'mousedown',
      'mouseup',
    ];

    for (const eventName of events) {
      let eventBubbled = false;
      const listener = () => {
        eventBubbled = true;
      };
      document.body.addEventListener(eventName, listener);

      // Dispatch event on the element
      const event = new Event(eventName, { bubbles: true, composed: true });
      element.dispatchEvent(event);

      expect(eventBubbled, `Event ${eventName} should not bubble`).to.be.false;

      document.body.removeEventListener(eventName, listener);
    }
  });

  it('renders unassociate bug button when bug_id is present', async () => {
    const rows = [{ commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' }];
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[0],
      x: 100,
      y: 100,
    };
    element.regressions = {
      test: {
        100: {
          id: '123',
          bugs: [{ bug_id: '456', bug_type: 'BUGANIZER' }],
          is_improvement: false,
          median_before: 5.0,
          median_after: 10.0,
        } as any,
      },
    };
    await element.updateComplete;

    const unassociateBtn = element.shadowRoot!.querySelector('#unassociate-bug-button');
    expect(unassociateBtn).to.not.be.null;
  });

  it('unassociates bug when unassociate button is clicked', async () => {
    const rows = [{ commit_number: 100, val: 10.0, createdat: 1000, hash: 'hash100' }];
    const regression = {
      id: '123',
      bugs: [{ bug_id: '456', bug_type: 'BUGANIZER' }],
      is_improvement: false,
      median_before: 5.0,
      median_after: 10.0,
    };
    element.hoveredPoint = {
      series: { id: 'test', color: '#fff', rows: rows },
      row: rows[0],
      x: 100,
      y: 100,
    };
    element.regressions = {
      test: {
        100: regression as any,
      },
    };
    await element.updateComplete;

    fetchMock.post('/_/triage/edit_anomalies', { status: 200, body: JSON.stringify({}) });

    let anomalyChangedDetail: any = null;
    element.addEventListener('anomaly-changed', (e) => {
      anomalyChangedDetail = (e as CustomEvent).detail;
    });

    const unassociateBtn = element.shadowRoot!.querySelector(
      '#unassociate-bug-button'
    ) as HTMLElement;
    expect(unassociateBtn).to.not.be.null;

    unassociateBtn.click();
    await fetchMock.flush(true);

    expect(anomalyChangedDetail).to.not.be.null;
    expect(anomalyChangedDetail.editAction).to.equal('RESET');
    expect(anomalyChangedDetail.traceNames).to.deep.equal(['test']);
    expect((regression as any).bug_id === 0 || regression.bugs?.length === 0).to.be.true;
  });
});
