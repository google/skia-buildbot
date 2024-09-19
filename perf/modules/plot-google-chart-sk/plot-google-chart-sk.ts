/**
 * @module modules/plot-google-chart-sk
 * @description <h2><code>plot-google-chart-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import '@google-web-components/google-chart';
import { GoogleChart } from '@google-web-components/google-chart';

import { html, css } from 'lit';
import { LitElement } from 'lit';
import { ref, Ref, createRef } from 'lit/directives/ref.js';
import { define } from '../../../elements-sk/modules/define';
import { Anomaly } from '../json';
import {
  ChartData,
  convertMainData,
  mainChartOptions,
} from '../common/plot-builder';

export interface AnomalyData {
  x: number;
  y: number;
  anomaly: Anomaly;
  highlight: boolean;
}

export class PlotGoogleChartSk extends LitElement {
  // TODO(b/362831653): Adjust height to 100% once plot-summary-sk is deprecated
  static styles = css`
    .plot {
      position: absolute;
      top: 0;
      left: 0;
      width: 100%;
      height: 45%;
    }
  `;

  constructor() {
    super();
  }

  // The div element that will host the plot on the summary.
  private plotElement: Ref<GoogleChart> = createRef();

  connectedCallback(): void {
    super.connectedCallback();

    const resizeObserver = new ResizeObserver(
      (entries: ResizeObserverEntry[]) => {
        entries.forEach(() => {
          // The google chart needs to redraw when it is resized.
          this.plotElement.value?.redraw();
        });
      }
    );
    resizeObserver.observe(this);
  }

  protected render() {
    return html`
      <google-chart ${ref(this.plotElement)} class="plot" type="line">
      </google-chart>
    `;
  }

  // Display the chart data on the plot.
  // TODO(b/362831653): Use dataframe to capture converted rows from `convertMainData`
  // TODO(b/362831653): Set updateChartData to private and use react property to auto-trigger
  public updateChartData(chartData: ChartData) {
    this.plotElement.value!.data = convertMainData(chartData);
    this.plotElement.value!.options = mainChartOptions(
      getComputedStyle(this),
      chartData
    );

    this.requestUpdate();
  }
}

define('plot-google-chart-sk', PlotGoogleChartSk);
