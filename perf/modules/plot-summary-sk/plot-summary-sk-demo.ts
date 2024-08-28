import './index';

import { $, $$ } from '../../../infra-sk/modules/dom';
import {
  PlotSummarySk,
  PlotSummarySkSelectionEventDetails,
} from './plot-summary-sk';
import { ChartAxisFormat, ChartData } from '../common/plot-builder';

document
  .querySelector('plot-summary-sk')!
  .addEventListener('summary_selected', (e) => {
    const plotDetails = (e as CustomEvent<PlotSummarySkSelectionEventDetails>)
      .detail;
    document.querySelector('#events')!.textContent = JSON.stringify(
      plotDetails,
      null,
      '  '
    );
  });

window.customElements.whenDefined('plot-summary-sk').then(() => {
  const chartData: ChartData = {
    xLabel: 'test x',
    yLabel: 'test y',
    chartAxisFormat: ChartAxisFormat.Date,
    lines: {
      test: [
        { x: new Date('2023/10/1'), y: 1, anomaly: false },
        { x: new Date('2023/10/2'), y: 1, anomaly: false },
        { x: new Date('2023/10/3'), y: 1, anomaly: false },
        { x: new Date('2023/10/4'), y: 4, anomaly: false },
        { x: new Date('2023/10/5'), y: 6, anomaly: false },
        { x: new Date('2023/10/6'), y: 3, anomaly: false },
        { x: new Date('2023/10/7'), y: 2, anomaly: false },
        { x: new Date('2023/10/8'), y: 1, anomaly: false },
        { x: new Date('2023/10/9'), y: 1, anomaly: false },
        { x: new Date('2023/10/10'), y: 1, anomaly: false },
        { x: new Date('2023/10/11'), y: 4, anomaly: false },
        { x: new Date('2023/10/12'), y: 6, anomaly: false },
        { x: new Date('2023/10/13'), y: 3, anomaly: false },
        { x: new Date('2023/10/14'), y: 2, anomaly: false },
        { x: new Date('2023/10/15'), y: 1, anomaly: false },
      ],
    },
    start: new Date('2023/10/1'),
    end: new Date('2023/10/15'),
  };
  $<PlotSummarySk>('plot-summary-sk').forEach((plot) => {
    plot.DisplayChartData(chartData, false);
  });
});
