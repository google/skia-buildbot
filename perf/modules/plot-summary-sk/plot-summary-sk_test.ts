import './index';

import { load } from '@google-web-components/google-chart/loader';
import { assert } from 'chai';
import { LitElement } from 'lit';
import sinon from 'sinon';

import { PlotSummarySk } from './plot-summary-sk';
import { PlotGoogleChartSk } from '../plot-google-chart-sk/plot-google-chart-sk';
import { generateFullDataFrame } from '../dataframe/test_utils';
import { convertFromDataframe, getTraceColor } from '../common/plot-builder';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('plot-summary-sk', () => {
  const now = new Date('2024/9/20').getTime();
  const timeSpans = [6 * 60 * 60]; // 6 hours
  const commitRange = { begin: 100, end: 110 };
  const df = generateFullDataFrame(commitRange, now, 1, timeSpans);
  const newEl = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  // `describe` doesn't support async setup, so we need to move this into `before` block.
  // The Google Chart API is being loaded async'ly.
  let dt: google.visualization.DataTable | null = null;
  before(async () => {
    // Load Google Chart API for DataTable.
    await load();
    dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);
  });

  const chartReady = (cb: () => LitElement) =>
    new Promise<LitElement>((resolve) => {
      const el = cb();
      el.addEventListener('google-chart-ready', () => {
        resolve(el);
      });
    }).then((el) => {
      return el.updateComplete;
    });

  describe('trace colors', () => {
    it('assigns deterministic colors based on trace name', async () => {
      const element = newEl((el) => {
        el.style.width = '100px';
      });
      await element.updateComplete;
      await chartReady(() => {
        element.data = dt;
        return element;
      });

      const traceKey = Object.keys(df.traceset)[0];
      const expectedColor = getTraceColor(traceKey);
      const actualColor = (element as any).traceColorMap.get(traceKey);

      assert.equal(actualColor, expectedColor);
    });
  });

  describe('trace colors consistency', () => {
    let fetchStub: sinon.SinonStub;

    beforeEach(() => {
      fetchStub = sinon.stub(window, 'fetch').resolves({
        ok: true,
        text: () => Promise.resolve(''),
        json: () => Promise.resolve({}),
      } as Response);
    });

    afterEach(() => {
      fetchStub.restore();
    });

    it('matches PlotGoogleChartSk color for the same trace', async () => {
      // Setup PlotSummarySk
      const summaryElement = newEl((el) => {
        el.style.width = '100px';
      });
      await summaryElement.updateComplete;
      await chartReady(() => {
        summaryElement.data = dt;
        return summaryElement;
      });

      // Setup PlotGoogleChartSk
      const googleChartElement = new PlotGoogleChartSk();
      document.body.appendChild(googleChartElement);
      googleChartElement.data = dt;
      await googleChartElement.updateComplete;
      // Allow async updateDataView to finish
      await new Promise((resolve) => setTimeout(resolve, 0));
      await googleChartElement.updateComplete;

      const traceKey = Object.keys(df.traceset)[0];
      const summaryColor = (summaryElement as any).traceColorMap.get(traceKey);
      const googleChartColor = googleChartElement.traceColorMap.get(traceKey);

      assert.equal(summaryColor, googleChartColor);

      document.body.removeChild(googleChartElement);
    });
  });

  ['material'].forEach((mode) =>
    describe(`selection in mode ${mode}`, () => {
      it('select an area', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
        });
        await element.updateComplete;
        await chartReady(() => {
          element.data = dt;
          return element;
        });

        const header = df.header!,
          start = 2,
          end = 6;
        element.Select(header![start]!, header![end]!);

        assert.approximately(element.selectedValueRange!.begin, header[start]!.offset, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.offset, 1e-3);
      });

      it('select an area before the chart is ready', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
        });
        await element.updateComplete;

        const header = df.header!,
          start = 2,
          end = 6;
        element.data = dt;
        element.Select(header![start]!, header![end]!);

        assert.isNull(element['chartLayout']);
        assert.approximately(element.selectedValueRange!.begin!, header[start]!.offset, 1e-3);
        assert.approximately(element.selectedValueRange!.end!, header[end]!.offset, 1e-3);

        await chartReady(() => {
          return element;
        });

        assert.approximately(element.selectedValueRange!.begin, header[start]!.offset, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.offset, 1e-3);
      });

      it('select an area in date mode', async () => {
        const element = newEl((el) => {
          el.style.width = '100px';
          // Enable date mode, the underlying data should stay the same.
          el.domain = 'date';
        });
        await element.updateComplete;
        await chartReady(() => {
          element.data = dt;
          return element;
        });

        const header = df.header!,
          start = 3,
          end = 9;
        element.Select(header![start]!, header![end]!);
        assert.approximately(element.selectedValueRange!.begin, header[start]!.timestamp, 1e-3);
        assert.approximately(element.selectedValueRange!.end, header[end]!.timestamp, 1e-3);
      });
    })
  );

  describe('performance with downsampling', () => {
    it('efficiently handles large datasets using min-max bucketing', async () => {
      const element = newEl((el) => {
        el.style.width = '100px';
      });
      await element.updateComplete;

      // Create a large dataset (e.g., 20,000 rows)
      const numRows = 20000;
      const data = new google.visualization.DataTable();
      data.addColumn('number', 'offset');
      data.addColumn('datetime', 'timestamp');
      data.addColumn('number', 'trace1');

      const dataRows = [];
      for (let i = 0; i < numRows; i++) {
        dataRows.push([i, new Date(now + i * 1000), Math.sin(i / 100) * 100]);
      }
      data.addRows(dataRows);

      const start = performance.now();

      // Trigger the update (which includes downsampling logic)
      element.data = data;
      // We access the private method directly or trigger it via property change.
      // Property 'data' change calls 'updateDataView'.

      // Wait for it to settle? updateDataView is synchronous in logic but might trigger async rendering.
      // The downsampling calculation happens synchronously in updateDataView.
      await element.updateComplete;

      const end = performance.now();
      const duration = end - start;
      assert.isBelow(duration, 200, 'Downsampling should be fast (< 200ms)');

      // Verify downsampling occurred by checking the view in the chart
      const view = (element as any)._viewForTesting;
      assert.isNotNull(view);
      const numDownsampledRows = view.getNumberOfRows();
      assert.isBelow(numDownsampledRows, numRows, 'Data should be downsampled');
      // We expect ~1000 rows (target resolution)
      assert.isBelow(numDownsampledRows, 1100, 'Downsampled size should be close to target (1000)');
      assert.isAbove(numDownsampledRows, 900, 'Downsampled size should be close to target (1000)');
    });
  });
});
