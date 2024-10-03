import './index';
import { $ } from '../../../infra-sk/modules/dom';
import { PlotGoogleChartSk } from './plot-google-chart-sk';
import { ChartAxisFormat, ChartData } from '../common/plot-builder';
import { Anomaly } from '../json';

document.querySelector('plot-google-chart-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});

const dummyAnomaly = (): Anomaly => ({
  id: 0,
  test_path: '',
  bug_id: -1,
  start_revision: 0,
  end_revision: 3,
  is_improvement: false,
  recovered: true,
  state: '',
  statistic: '',
  units: '',
  degrees_of_freedom: 0,
  median_before_anomaly: 0,
  median_after_anomaly: 0,
  p_value: 0,
  segment_size_after: 0,
  segment_size_before: 0,
  std_dev_before_anomaly: 0,
  t_statistic: 0,
  subscription_name: '',
  bug_component: '',
  bug_labels: null,
  bug_cc_emails: null,
});

window.customElements.whenDefined('plot-google-chart-sk').then(() => {
  const chartData: ChartData = {
    xLabel: 'Dates',
    yLabel: 'Regression Values',
    chartAxisFormat: ChartAxisFormat.Date,
    // It's assumed that the length of x axis values will always be consistent
    // meaning that because all traces aren't generated daily, there could be
    // a lot of NaN values as a result of MISSING_DATA_SENTINEL.
    lines: {
      trace_1: [
        { x: new Date('2023/10/1'), y: 1, anomaly: null },
        { x: new Date('2023/10/2'), y: NaN, anomaly: null },
        { x: new Date('2023/10/3'), y: 1, anomaly: null },
        { x: new Date('2023/10/4'), y: 4, anomaly: null },
        { x: new Date('2023/10/5'), y: 6, anomaly: null },
        { x: new Date('2023/10/6'), y: 3, anomaly: null },
        { x: new Date('2023/10/7'), y: 2, anomaly: null },
        { x: new Date('2023/10/8'), y: 1, anomaly: null },
        { x: new Date('2023/10/9'), y: 1, anomaly: null },
        { x: new Date('2023/10/10'), y: 1, anomaly: null },
        { x: new Date('2023/10/11'), y: 4, anomaly: null },
        { x: new Date('2023/10/12'), y: 6, anomaly: dummyAnomaly() },
        { x: new Date('2023/10/13'), y: 3, anomaly: null },
        { x: new Date('2023/10/14'), y: 2, anomaly: null },
        { x: new Date('2023/10/15'), y: 1, anomaly: null },
      ],
      trace_2: [
        { x: new Date('2023/10/1'), y: 2, anomaly: null },
        { x: new Date('2023/10/2'), y: 2, anomaly: null },
        { x: new Date('2023/10/3'), y: 2, anomaly: null },
        { x: new Date('2023/10/4'), y: 4, anomaly: null },
        { x: new Date('2023/10/5'), y: 6, anomaly: null },
        { x: new Date('2023/10/6'), y: 4, anomaly: null },
        { x: new Date('2023/10/7'), y: 4, anomaly: null },
        { x: new Date('2023/10/8'), y: 4, anomaly: null },
        { x: new Date('2023/10/9'), y: 1, anomaly: null },
        { x: new Date('2023/10/10'), y: NaN, anomaly: null },
        { x: new Date('2023/10/11'), y: NaN, anomaly: null },
        { x: new Date('2023/10/12'), y: NaN, anomaly: null },
        { x: new Date('2023/10/13'), y: NaN, anomaly: null },
        { x: new Date('2023/10/14'), y: 9, anomaly: dummyAnomaly() },
        { x: new Date('2023/10/15'), y: 1, anomaly: null },
      ],
    },
    start: new Date('2023/10/1'),
    end: new Date('2023/10/15'),
  };
  $<PlotGoogleChartSk>('plot-google-chart-sk').forEach((plot) => {
    plot.updateChartData(chartData);
  });
});
