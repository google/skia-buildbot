/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import { PlotSummarySk } from './plot-summary-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ChartAxisFormat, ChartData } from '../common/plot-builder';
import { ColumnHeader } from '../json';
import { LitElement } from 'lit';

describe('plot-summary-sk', () => {
  const newInstance = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  let element: PlotSummarySk;

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
    beforeEach(() => {
      element = newInstance();
    });

    it('Select an area', async () => {
      const chartData: ChartData = {
        lines: {
          test: [
            { x: 1, y: 1, anomaly: null },
            { x: 2, y: 2, anomaly: null },
            { x: 3, y: 3, anomaly: null },
            { x: 4, y: 4, anomaly: null },
            { x: 5, y: 5, anomaly: null },
            { x: 6, y: 6, anomaly: null },
            { x: 7, y: 7, anomaly: null },
            { x: 8, y: 8, anomaly: null },
            { x: 9, y: 9, anomaly: null },
          ],
        },
        chartAxisFormat: ChartAxisFormat.Commit,
        xLabel: 'xLabel',
        yLabel: 'yLabel',
        start: 1,
        end: 9,
      };
      element.style.width = '10px';
      element.style.display = 'inline-block';
      element.requestUpdate();
      await element.updateComplete;

      element.DisplayChartData(chartData, true);
      await element.updateComplete;
      await chartReady(() => element);
      element.Select(
        { offset: 3 } as ColumnHeader,
        { offset: 7 } as ColumnHeader
      );

      // Because of how d3scale works, we will not get the exact
      // values in the event
      const selectionRange = element['selectionRange'];
      assert.approximately(3, Math.floor(selectionRange![0]), 1);
      assert.approximately(7, Math.floor(selectionRange![1]), 1);
    });
  });
});
