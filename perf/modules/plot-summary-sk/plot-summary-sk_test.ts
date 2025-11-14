import './index';

import { load } from '@google-web-components/google-chart/loader';
import { assert } from 'chai';
import { LitElement } from 'lit';

import { PlotSummarySk } from './plot-summary-sk';
import { generateFullDataFrame } from '../dataframe/test_utils';
import { convertFromDataframe } from '../common/plot-builder';
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
});
