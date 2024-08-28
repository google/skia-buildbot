/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import {
  PlotSummarySk,
  PlotSummarySkSelectionEventDetails,
} from './plot-summary-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ChartAxisFormat, ChartData } from '../common/plot-builder';

describe('plot-summary-sk', () => {
  const newInstance = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  let element: PlotSummarySk;

  describe('Selection', () => {
    let lastEvent: PlotSummarySkSelectionEventDetails;
    beforeEach(() => {
      element = newInstance((el: PlotSummarySk) => {
        el.addEventListener('summary_selected', (e) => {
          lastEvent = (e as CustomEvent<PlotSummarySkSelectionEventDetails>)
            .detail;
        });
      });
    });

    it('Select an area', () => {
      const chartData: ChartData = {
        lines: {
          test: [
            { x: 1, y: 1, anomaly: false },
            { x: 2, y: 2, anomaly: false },
            { x: 3, y: 3, anomaly: false },
            { x: 4, y: 4, anomaly: false },
            { x: 5, y: 5, anomaly: false },
            { x: 6, y: 6, anomaly: false },
            { x: 7, y: 7, anomaly: false },
            { x: 8, y: 8, anomaly: false },
            { x: 9, y: 9, anomaly: false },
          ],
        },
        chartAxisFormat: ChartAxisFormat.Commit,
        xLabel: 'xLabel',
        yLabel: 'yLabel',
        start: 1,
        end: 9,
      };
      element.width = 10;
      element.DisplayChartData(chartData, true);
      element.Select(3, 7);

      // Because of how d3scale works, we will not get the exact
      // values in the event
      const selectionRange = element['selectionRange'];
      assert.isTrue(Math.abs(3 - Math.floor(selectionRange![0])) <= 1);
      assert.isTrue(Math.abs(7 - Math.ceil(selectionRange![1])) <= 1);
    });
  });
});
