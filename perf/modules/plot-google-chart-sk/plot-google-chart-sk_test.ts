import './index';
import { assert } from 'chai';
import { PlotGoogleChartSk } from './plot-google-chart-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { getTraceColor } from '../common/plot-builder';
import sinon from 'sinon';

// Mock the google.visualization object.
const createMockDataTable = () => ({
  getNumberOfRows: () => 3,
  getNumberOfColumns: () => 2,
  getValue: (rowIndex: number, colIndex: number) => {
    if (colIndex === 0) {
      return rowIndex + 1;
    }
    if (colIndex === 1) {
      return new Date(2025, 1, rowIndex + 1);
    }
    return (rowIndex + 1) * 10;
  },
  getFilteredRows: (_filters: any[]) => {
    const rows = [];
    for (let i = 0; i < 3; i++) {
      rows.push(i);
    }
    return rows;
  },
  getColumnLabel: (colIndex: number) => `Col ${colIndex}`,
  getViewColumns: () => [0, 1, 2],
  getViewColumnIndex: (colIndex: number) => colIndex,
  getColumnIndex: (_label: string) => 0,
});

// Mock google chart object
const createMockChart = () => ({
  getChartLayoutInterface: () => ({
    getChartAreaBoundingBox: () => ({ top: 0, left: 0, width: 100, height: 100 }),
    getVAxisValue: (y: number) => y,
    getHAxisValue: (x: number) => x,
    getXLocation: (x: number) => x,
    getYLocation: (y: number) => y,
  }),
  setSelection: () => {},
  getSelection: () => [],
  draw: () => {},
  clearChart: () => {},
});

describe('plot-google-chart-sk', () => {
  const newInstance = setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
  let element: PlotGoogleChartSk;
  let originalGoogle: any;

  beforeEach(() => {
    originalGoogle = window.google;
    element = newInstance(() => {});
    // @ts-expect-error - mocking google.visualization
    window.google = {
      visualization: {
        DataTable: sinon.stub().callsFake(createMockDataTable),
        DataView: sinon.stub().callsFake(createMockDataTable),
        LineChart: sinon.stub().callsFake(createMockChart),
        events: {
          addListener: sinon.stub(),
          trigger: sinon.stub(),
        },
      } as any,
    };
  });

  afterEach(() => {
    if (originalGoogle) {
      window.google = originalGoogle;
    } else {
      // @ts-expect-error - clean up global mock
      delete window.google;
    }
  });

  describe('trace colors', () => {
    it('assigns deterministic colors based on trace name', async () => {
      const traceName = 'trace_A';
      const expectedColor = getTraceColor(traceName);

      // Override the default mock to return our specific trace name
      const mockDataTable = createMockDataTable();
      mockDataTable.getNumberOfColumns = () => 3;
      mockDataTable.getColumnLabel = (colIndex: number) => (colIndex === 2 ? traceName : 'Domain');

      // Mock DataView constructor
      const MockDataView = function () {
        return mockDataTable;
      } as any;

      (window.google.visualization as any).DataTable = sinon.stub().returns(mockDataTable);

      window.google.visualization.DataView = MockDataView;

      element.data = new google.visualization.DataTable();
      await element.updateComplete;

      // updateDataView is async and runs after the first update.
      // We yield to the event loop to allow it to run and update traceColorMap.
      await new Promise((resolve) => setTimeout(resolve, 0));
      await element.updateComplete;

      assert.equal(element.traceColorMap.get(traceName), expectedColor);
    });
  });

  describe('determineYAxisTitle', () => {
    // trace samples for determineYAxisTitle unit tests
    const ms_down = 'unit=ms,improvement_direction=down';
    const ms_up = 'unit=ms,improvement_direction=up';
    const score_down = 'unit=score,improvement_direction=down';

    it('returns empty string for empty input', () => {
      assert.isEmpty(element.determineYAxisTitle([]));
    });

    it('returns formatted title when unit and improvement direction are same', () => {
      assert.strictEqual(element.determineYAxisTitle([ms_down, ms_down]), 'ms - down');
    });

    it('returns unit only when improvement direction differs', () => {
      assert.strictEqual(element.determineYAxisTitle([ms_down, ms_up]), 'ms');
    });

    it('returns direction only when unit differs', () => {
      assert.strictEqual(element.determineYAxisTitle([ms_down, score_down]), 'down');
    });

    it('returns empty string when all differ', () => {
      assert.isEmpty(element.determineYAxisTitle([ms_up, score_down]));
    });
  });

  describe('domain change conversion', () => {
    it('converts selection range from date to commit', async () => {
      element.data = new google.visualization.DataTable();
      element.domain = 'date';
      element.selectedRange = {
        begin: new Date(2025, 1, 1).getTime() / 1000,
        end: new Date(2025, 1, 3).getTime() / 1000,
      };
      await element.updateComplete;

      element.domain = 'commit';
      await element.updateComplete;

      assert.equal(element.selectedRange!.begin, 1);
      assert.equal(element.selectedRange!.end, 3);
    });

    it('converts selection range from commit to date', async () => {
      element.data = new google.visualization.DataTable();
      element.domain = 'commit';
      element.selectedRange = {
        begin: 1,
        end: 3,
      };
      await element.updateComplete;

      element.domain = 'date';
      await element.updateComplete;

      assert.equal(element.selectedRange!.begin, new Date(2025, 1, 1).getTime() / 1000);
      assert.equal(element.selectedRange!.end, new Date(2025, 1, 3).getTime() / 1000);
    });
  });

  describe('hidden attribute', () => {
    it('is present when there is no data', async () => {
      element.data = null;
      await element.updateComplete;
      const chart = element.shadowRoot!.querySelector('google-chart');
      assert.isTrue(chart!.hasAttribute('hidden'));
    });

    it('is not present when there is data', async () => {
      element.data = new google.visualization.DataTable();
      await element.updateComplete;
      const chart = element.shadowRoot!.querySelector('google-chart');
      assert.isFalse(chart!.hasAttribute('hidden'));
    });
  });

  describe('vertical zoom persistence', () => {
    it('preserves vertical zoom when horizontal zoom changes', async () => {
      // Setup data
      element.data = new google.visualization.DataTable();
      await element.updateComplete;

      // 1. Set Vertical Zoom
      element.isHorizontalZoom = false; // Vertical zoom mode
      element.updateBounds({ begin: 10, end: 50 });

      // Verify vertical zoom is set in state
      const zoomedVRange = (element as any).zoomedVRange;
      assert.isNotNull(zoomedVRange);
      assert.equal(zoomedVRange.min, 10);
      assert.equal(zoomedVRange.max, 50);

      // 2. Change Horizontal Zoom (selectedRange)
      element.selectedRange = { begin: 5, end: 25 };
      await element.updateComplete;

      // Vertical zoom should persist in state
      const persistedVRange = (element as any).zoomedVRange;
      assert.isNotNull(persistedVRange);
      assert.equal(persistedVRange.min, 10, 'Vertical zoom min should persist');
      assert.equal(persistedVRange.max, 50, 'Vertical zoom max should persist');
    });

    it('preserves horizontal zoom logic when vertical zoom changes', async () => {
      // Setup data
      element.data = new google.visualization.DataTable();
      await element.updateComplete;

      // 1. Set Horizontal Zoom (selectedRange)
      element.selectedRange = { begin: 100, end: 200 };
      await element.updateComplete;

      // Verify no vertical zoom initially
      assert.isNull((element as any).zoomedVRange);

      // 2. Change Vertical Zoom
      element.isHorizontalZoom = false; // Vertical zoom mode
      element.updateBounds({ begin: 10, end: 50 });

      // Verify vertical zoom is updated in state
      const zoomedVRange = (element as any).zoomedVRange;
      assert.isNotNull(zoomedVRange);
      assert.equal(zoomedVRange.min, 10);
      assert.equal(zoomedVRange.max, 50);

      // Verify horizontal zoom (selectedRange) is preserved
      assert.equal(element.selectedRange!.begin, 100);
      assert.equal(element.selectedRange!.end, 200);

      // 3. Update Horizontal Zoom again (simulate user action or data update)
      element.selectedRange = { begin: 150, end: 250 };
      await element.updateComplete;

      // Vertical zoom state should persist
      const persistedVRange = (element as any).zoomedVRange;
      assert.isNotNull(persistedVRange);
      assert.equal(persistedVRange.min, 10);
      assert.equal(persistedVRange.max, 50);
    });
  });
});
