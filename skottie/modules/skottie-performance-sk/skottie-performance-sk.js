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
import { html, render } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { Chart } from 'chart.js';

const marks = {
  START: 'Start Skottie Frame',
  END: 'End Skottie Frame',
  NAME: 'Skottie Frame',
};

const template = () => html`
  <div>
    <div class="chart">
      <canvas id=performance-chart height=400 width=400></canvas>
    </div>
  </div>
`;


class SkottiePerformanceSk extends HTMLElement {
  constructor() {
    super();
    this._currentMeasuringFrame = -1;
    this._currentMetrics = [];
  }

  _createNewChart() {
    if (this._chart) {
      this._chart.destroy();
    }
    const canvas = $$('#performance-chart', this);
    const ctx = canvas.getContext('2d');
    this._chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: [],
        datasets: [
          {
            label: 'Frame Duration (ms)',
            fillColor: 'rgba(220,220,220,0.2)',
            strokeColor: 'rgba(220,220,220,1)',
            pointColor: 'rgba(220,220,220,1)',
            pointStrokeColor: '#fff',
            data: [],
          },
        ],
      },
      options: {
        maintainAspectRatio: false,
      },
    });
  }

  connectedCallback() {
    this._render();
    this._createNewChart();
  }

  reset() {
    this._createNewChart();
  }

  _addDataPoint(frame, metrics) {
    if (!this._chart) {
      return;
    }
    if (!this._currentMetrics[frame]) {
      this._currentMetrics[frame] = {
        values: [],
        average: 0,
      };
    }
    const metricData = this._currentMetrics[frame];
    metrics.forEach((metric) => metricData.values.push(metric.duration));
    const average =
      metricData.values.reduce((acc, value) => acc + value, 0) / metricData.values.length;
    if (!this._chart.data.labels[frame]) {
      while (!this._chart.data.labels[frame]) {
        this._chart.data.labels.push(`frame ${this._chart.data.labels.length}`);
      }
    }
    this._chart.data.datasets[0].data[frame] = average;
    this._chart.update();
  }

  start(progress, duration, fps) {
    const newFrame = Math.floor((progress * fps) / 1000);
    if (this._currentMeasuringFrame !== newFrame) {
      this._currentMeasuringFrame = newFrame;
      const metrics = performance.getEntriesByName(marks.NAME);
      this._addDataPoint(newFrame, metrics);
      performance.clearMarks();
      performance.clearResourceTimings();
      performance.clearMeasures();
    }
    performance.mark(marks.START);
  }

  end() {
    performance.mark(marks.END);
    performance.measure(marks.NAME, marks.START, marks.END);
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
}

define('skottie-performance-sk', SkottiePerformanceSk);
