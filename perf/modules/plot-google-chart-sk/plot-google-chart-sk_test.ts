import './index';
import { assert, expect } from 'chai';
import { PlotGoogleChartSk } from './plot-google-chart-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import sinon from 'sinon';

// Mock the google.visualization object.
const mockDataTable = {
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
};

// TODO(b/362831653): Add unit tests for plot-google-chart
describe('plot-google-chart-sk', () => {
  // trace samples for determineYAxisTitle unit tests
  const ms_down = 'unit=ms,improvement_direction=down';
  const ms_up = 'unit=ms,improvement_direction=up';
  const score_down = 'unit=score,improvement_direction=down';

  const newInstance = setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
  let element: PlotGoogleChartSk;

  beforeEach(() => {
    element = newInstance(() => {});
    // @ts-expect-error - mocking google.visualization
    window.google = {
      visualization: {
        DataTable: sinon.stub().returns(mockDataTable),
      } as any,
    };
  });

  describe('some action', () => {
    it('some result', () => {
      // eslint-disable-next-line @typescript-eslint/no-unused-expressions
      expect(element).to.not.be.null;
    });

    it('some result', () => {
      // eslint-disable-next-line @typescript-eslint/no-unused-expressions
      expect(element).to.not.be.null;
    });
  });

  describe('determineYAxisTitle', () => {
    it('empty', () => {
      assert.isEmpty(element.determineYAxisTitle([]));
    });

    it('unit and improvement direction same', () => {
      assert.strictEqual('ms - down', element.determineYAxisTitle([ms_down, ms_down]));
    });

    it('unit same, improvement direction different', () => {
      assert.strictEqual('ms', element.determineYAxisTitle([ms_down, ms_up]));
    });

    it('unit different, improvement direction same', () => {
      assert.strictEqual('down', element.determineYAxisTitle([ms_down, score_down]));
    });

    it('all different', () => {
      assert.isEmpty(element.determineYAxisTitle([ms_up, score_down]));
    });
  });

  describe('willUpdate', () => {
    it('converts selection range from date to commit', () => {
      element.data = new google.visualization.DataTable();
      element.domain = 'commit';
      element.selectedRange = {
        begin: new Date(2025, 1, 1).getTime() / 1000,
        end: new Date(2025, 1, 3).getTime() / 1000,
      };

      const changedProperties = new Map();
      changedProperties.set('domain', 'date');
      (element as any).willUpdate(changedProperties);

      expect(element.selectedRange.begin).to.equal(1);
      expect(element.selectedRange.end).to.equal(3);
    });

    it('converts selection range from commit to date', () => {
      element.data = new google.visualization.DataTable();
      element.domain = 'date';
      element.selectedRange = {
        begin: 1,
        end: 3,
      };

      const changedProperties = new Map();
      changedProperties.set('domain', 'commit');
      (element as any).willUpdate(changedProperties);

      expect(element.selectedRange.begin).to.equal(new Date(2025, 1, 1).getTime() / 1000);
      expect(element.selectedRange.end).to.equal(new Date(2025, 1, 3).getTime() / 1000);
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
});
