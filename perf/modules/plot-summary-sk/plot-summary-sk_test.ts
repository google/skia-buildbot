import './index';
import { assert } from 'chai';
import { PlotSummarySk } from './plot-summary-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { generateFullDataFrame } from '../dataframe/test_utils';
import { LitElement } from 'lit';

describe('plot-summary-sk', () => {
  const now = new Date('2024/9/20').getTime();
  const timeSpans = [6 * 60 * 60]; // 6 hours
  const commitRange = { begin: 100, end: 110 };
  const df = generateFullDataFrame(commitRange, now, 1, timeSpans);
  const newEl = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  const chartReady = (cb: () => LitElement) =>
    new Promise<LitElement>((resolve) => {
      const el = cb();
      el.addEventListener('google-chart-ready', () => {
        resolve(el);
      });
    }).then((el) => {
      return el.updateComplete;
    });

  describe('Selection', () => {
    it('select an area in default canvas mode', async () => {
      const element = newEl((el) => {
        el.style.width = '10px';
      });
      await element.updateComplete;
      await chartReady(() => {
        element.dataframe = df;
        return element;
      });

      const header = df.header!,
        start = 2,
        end = 4;
      element.Select(header![start]!, header![end]!);

      const selectionRange = element['selectionRange'];
      assert.approximately(start, Math.floor(selectionRange![0]), 1);
      assert.approximately(end, Math.floor(selectionRange![1]), 1);
    });

    it('select an area before the chart is ready', async () => {
      const element = newEl((el) => {
        el.style.width = '10px';
      });
      await element.updateComplete;

      const header = df.header!,
        start = 2,
        end = 4;
      element.dataframe = df;
      element.Select(header![start]!, header![end]!);

      assert.isNull(element['chartLayout']);
      assert.approximately(0, Math.floor(element['selectionRange']![0]), 1);

      await chartReady(() => {
        return element;
      });

      assert.approximately(2, Math.floor(element['selectionRange']![0]), 1);
      assert.approximately(4, Math.floor(element['selectionRange']![1]), 1);
    });

    it('select an area in date mode', async () => {
      const element = newEl((el) => {
        el.style.width = '10px';
        // Enable date mode, the underlying data should stay the same.
        el.domain = 'date';
      });
      await element.updateComplete;
      await chartReady(() => {
        element.dataframe = df;
        return element;
      });

      const header = df.header!,
        start = 3,
        end = 9;
      element.Select(header![start]!, header![end]!);
      assert.approximately(start, Math.floor(element['selectionRange']![0]), 1);
      assert.approximately(end, Math.floor(element['selectionRange']![1]), 1);
    });
  });
});
