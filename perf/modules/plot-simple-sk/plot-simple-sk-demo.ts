import './index';
import { $, $$ } from '../../../infra-sk/modules/dom';
import {
  AnomalyData,
  PlotSimpleSk,
  PlotSimpleSkTraceEventDetails,
  PlotSimpleSkZoomEventDetails,
} from './plot-simple-sk';
import { Anomaly } from '../json';
import '../../../infra-sk/modules/theme-chooser-sk';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { ticks } from './ticks';

// Create our own random number generator that's deterministic so that we get
// consistent Gold images.
let seed = 1;
const MAX = 1e20;
const random = (): number => {
  seed = (seed * 999331) /* a prime number */ % MAX;
  return seed / MAX;
};

const dummyAnomaly = (isImprovement: boolean): Anomaly => ({
  id: 0,
  test_path: '',
  bug_id: 123456,
  start_revision: 0,
  end_revision: 3,
  is_improvement: isImprovement,
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
  bug_labels: [],
  bug_cc_emails: [],
});

window.customElements.whenDefined('plot-simple-sk').then(() => {
  const ele = $$<PlotSimpleSk>('#plot')!;
  let n = 0;
  let traces: { [name: string]: number[] } = {};

  function add(plot: PlotSimpleSk, num: number) {
    const labels = [];
    for (let i = 0; i < 50; i++) {
      labels.push(new Date(1554143900000 + i * i * 5 * 1000 * 60));
    }

    traces = {};
    for (let j = 0; j < num; j++) {
      const trace = [];
      for (let i = 0; i < 50; i++) {
        if (random() < 0.9) {
          trace.push(1000000 * (8 + Math.sin(i / 10) + j + random() * 1 + 10));
        } else {
          trace.push(MISSING_DATA_SENTINEL);
        }
      }
      const id = `trace${j + n}`;
      traces[id] = trace;
    }

    n += num;
    plot.addLines(traces, ticks(labels));
  }

  $<PlotSimpleSk>('plot-simple-sk').forEach((plot) => {
    add(plot, 10);
  });

  $$<HTMLButtonElement>('#add')!.addEventListener('click', () => {
    add(ele, 10);
  });

  $$<HTMLButtonElement>('#addalot')!.addEventListener('click', () => {
    add(ele, 100);
  });

  $$<HTMLButtonElement>('#clear')!.addEventListener('click', () => {
    ele.removeAll();
  });

  $$<HTMLButtonElement>('#reset')!.addEventListener('click', () => {
    ele.zoom = null;
  });

  $$<HTMLButtonElement>('#high')!.addEventListener('click', () => {
    ele.highlight = ['trace0', 'trace1'];
  });

  $$<HTMLButtonElement>('#clearhigh')!.addEventListener('click', () => {
    ele.highlight = [];
  });

  $$<HTMLButtonElement>('#xbar')!.addEventListener('click', () => {
    ele.xbar = 3;
  });

  $$<HTMLButtonElement>('#clearxbar')!.addEventListener('click', () => {
    ele.xbar = -1;
  });

  $$<HTMLButtonElement>('#zoomAction')!.addEventListener('click', () => {
    ele.zoom = [20, 40];
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_selected', (e) => {
    $$('#selected')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('trace_focused', (e) => {
    $$('#focused')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkTraceEventDetails>).detail
    );
  });

  $$<PlotSimpleSk>('#plot')!.addEventListener('zoom', (e) => {
    $$('#zoom')!.textContent = JSON.stringify(
      (e as CustomEvent<PlotSimpleSkZoomEventDetails>).detail
    );
  });

  $$<HTMLButtonElement>('#bands')!.addEventListener('click', () => {
    ele.bands = [1, 4, 20, 30];
  });

  $$<HTMLButtonElement>('#toggleDots')!.addEventListener('click', () => {
    ele.dots = !ele.dots;
  });

  $$<HTMLButtonElement>('#special')!.addEventListener('click', () => {
    const trace = [];
    for (let i = 0; i < 50; i++) {
      trace.push(0);
    }
    ele.addLines({ specialZero: trace }, []);
  });

  $$<HTMLButtonElement>('#anomaly')!.addEventListener('click', () => {
    const anomalyDataMap: { [key: string]: AnomalyData[] } = {};
    const keys = Object.keys(traces);
    if (keys.length > 0) {
      const id = keys[0];
      anomalyDataMap[id] = [
        {
          x: 5,
          y: traces[id][5],
          anomaly: dummyAnomaly(false),
          highlight: true,
        },
        {
          x: 20,
          y: traces[id][20],
          anomaly: dummyAnomaly(true),
          highlight: false,
        },
      ];
    }
    ele.anomalyDataMap = anomalyDataMap;
  });

  $$<HTMLButtonElement>('#clearanomaly')!.addEventListener('click', () => {
    ele.anomalyDataMap = {};
  });
});
