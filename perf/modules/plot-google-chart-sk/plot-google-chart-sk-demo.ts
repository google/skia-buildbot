import './index';
import { $ } from '../../../infra-sk/modules/dom';
import { PlotGoogleChartSk } from './plot-google-chart-sk';
import { ChartAxisFormat, ChartData } from '../common/plot-builder';
import {
  Anomaly,
  ColumnHeader,
  CommitNumber,
  DataFrame,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
} from '../json';
import { html, LitElement, TemplateResult } from 'lit';
import { customElement } from 'lit/decorators.js';
import { DataFrameRepository } from '../dataframe/dataframe_context';

document.querySelector('plot-google-chart-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});

const dummyAnomaly = (): Anomaly => ({
  id: '0',
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
  bisect_ids: [],
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

//side-panel-sk demo data setup
// prettier-ignore
const TestData = {
  traceset: {
    ',arch=x86,config=8888,test=a,': [1, 2, 3],
    ',arch=x86,config=565,test=b,': [4, 5, 6],
    ',arch=arm,config=8888,test=c,': [7, 8, 9],
  },
};

@customElement('side-panel-sk-demo')
export class SidePanelSkDemo extends LitElement {
  private dfRepo: DataFrameRepository | null = null;

  render(): TemplateResult {
    return html`
      <dataframe-repository-sk>
        <side-panel-sk style="width: 250px; height: 300px;"></side-panel-sk>
      </dataframe-repository-sk>
    `;
  }

  firstUpdated() {
    this.dfRepo = this.shadowRoot!.querySelector<DataFrameRepository>('dataframe-repository-sk');
    if (this.dfRepo) {
      const traceset: TraceSet = TraceSet({});
      const header: ColumnHeader[] = [];

      Object.keys(TestData.traceset).forEach((key, index) => {
        traceset[key] = Trace(TestData.traceset[key as keyof typeof TestData.traceset]);
        // Create dummy ColumnHeaders for the demo
        header.push({
          offset: CommitNumber(index),
          timestamp: TimestampSeconds(Date.now() / 1000 + index * 60),
          hash: `hash_${index}`,
          author: `author_${index}`,
          message: `message_${index}`,
          url: `url_${index}`,
        });
      });

      const dummyDataFrame: DataFrame = {
        traceset: traceset,
        header: header,
        paramset: ReadOnlyParamSet({}),
        skip: 0,
        traceMetadata: [],
      };
      this.dfRepo.dataframe = dummyDataFrame;
    }
  }
}

// Log events for debugging in the browser.
window.addEventListener('side-panel-toggle', (e) => {
  console.log('side-panel-toggle event:', (e as CustomEvent).detail);
});
window.addEventListener('side-panel-selected-trace-change', (e) => {
  console.log('side-panel-selected-trace-change event:', (e as CustomEvent).detail);
});
