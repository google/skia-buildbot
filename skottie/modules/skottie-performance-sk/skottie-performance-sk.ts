/**
 * @module skottie-performance-sk
 * @description <h2><code>skottie-performance-sk</code></h2>
 *
 * <p>
 *   A skottie performance graph.
 *   It exposes two methods start and end to calculate rendering time of each frame
 * </p>
 *
 *
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import {
  Chart,
  BarController,
  CategoryScale,
  LinearScale,
  BarElement,
} from 'chart.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

Chart.register(
  BarElement,
  BarController,
  CategoryScale,
  LinearScale,
);

const marks = {
  START: 'Start Skottie Frame',
  END: 'End Skottie Frame',
  NAME: 'Skottie Frame',
};

interface FrameMetric {
  values: number[];
  average: number;
}

export class SkottiePerformanceSk extends ElementSk {
  private static template = () => html`
  <div>
    <div class="chart">
      <canvas id=performance-chart height=400 width=400></canvas>
    </div>
  </div>
`;

  private currentMeasuringFrame: number = -1;

  private currentMetrics: FrameMetric[] = [];

  private chart: Chart | null = null;

  constructor() {
    super(SkottiePerformanceSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._createNewChart();
  }

  disconnectedCallback() {
    if (this.chart) {
      this.chart.destroy();
    }
    super.disconnectedCallback();
  }

  _createNewChart() {
    if (this.chart) {
      this.currentMetrics = [];
      this.chart.destroy();
    }
    const canvas = $$<HTMLCanvasElement>('#performance-chart', this)!;
    const ctx = canvas.getContext('2d')!;
    this.chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: [],
        datasets: [
          {
            label: 'Frame Duration (ms)',
            backgroundColor: 'rgba(0,220,220,0.4)',
            borderColor: 'rgba(0,220,220,1)',
            data: [],
          },
        ],
      },
      options: {
        maintainAspectRatio: false,
      },
    });
  }

  reset() {
    this._createNewChart();
  }

  private addDataPoint(frame: number, metrics: PerformanceEntryList) {
    if (!this.chart) {
      return;
    }
    if (!this.currentMetrics[frame]) {
      this.currentMetrics[frame] = {
        values: [],
        average: 0,
      };
    }
    const metricData = this.currentMetrics[frame];
    metrics.forEach((metric: PerformanceEntry) => metricData.values.push(metric.duration));
    const average = metricData.values.reduce((acc: number, value) => acc + value, 0) / metricData.values.length;
    if (!this.chart.data.labels![frame]) {
      while (!this.chart.data.labels![frame]) {
        this.chart.data.labels!.push(`frame ${this.chart.data.labels!.length}`);
      }
    }
    this.chart.data.datasets[0].data[frame] = average;
    this.chart.update();
  }

  start(progress: number, duration: number, fps: number) {
    const newFrame = Math.floor((progress * fps) / 1000);
    if (this.currentMeasuringFrame !== newFrame) {
      this.currentMeasuringFrame = newFrame;
      const metrics = performance.getEntriesByName(marks.NAME);
      this.addDataPoint(newFrame, metrics);
      performance.clearMarks();
      performance.clearResourceTimings();
      performance.clearMeasures();
    }
    performance.mark(marks.START);
  }

  end(): void {
    performance.mark(marks.END);
    performance.measure(marks.NAME, marks.START, marks.END);
  }
}

define('skottie-performance-sk', SkottiePerformanceSk);
