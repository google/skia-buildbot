/**
 * @module bugs-chart-sk
 * @description <h2><code>bugs-chart-sk</code></h2>
 *
 * Displays a google-chart for the specified type and data.
 *
 * @attr chart_type {string} The type of the chart. Eg: open/slo.
 *
 * @attr chart_title {string} The title of the chart.
 *
 * @attr data {string} Data for the chart. Eg: '[["Month", "Days"], ["Jan", 31], ["Feb", 28], ["Mar", 31]]'.
 *
 */

import '@google-web-components/google-chart';

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export type ChartType = 'open' | 'slo' | 'untriaged';

function getChartOptions(type: string): string {
  const displayType = type === 'open' ? 'area' : 'line';
  const isStacked = type === 'open';
  return JSON.stringify({
    chartArea: {
      top: 15,
      left: 50,
      width: '83%',
      height: '82%',
    },
    backgroundColor: {
      fill: 'var(--background)',
      fillOpacity: 0.8,
    },
    hAxis: {
      slantedText: false,
    },
    legend: {
      position: 'bottom',
    },
    type: displayType,
    isStacked: isStacked,
    series: {
      3: {
        color: '#64c1f6',
      },
      2: {
        color: '#64B5F6',
      },
      1: {
        color: '#1E88E5',
      },
      0: {
        color: '#0D47A1',
      },
    },
  });
}

export class BugsChartSk extends ElementSk {
  constructor() {
    super(BugsChartSk.template);
  }

  private static template = (el: BugsChartSk) => html`
    <div class="charts-title">${el.chart_title}</div>
    <google-chart
      id="${el.chart_type}-chart"
      options="${getChartOptions(el.chart_type)}"
      data="${el.data}">
    </google-chart>
`;

  /** Reflects chart_type attribute for convenience. */
  get chart_type(): string { return this.getAttribute('chart_type')!; }

  set chart_type(val: string) { this.setAttribute('chart_type', (+val as unknown) as string); }

  /** Reflects chart_type attribute for convenience. */
  get chart_title(): string { return this.getAttribute('chart_title')!; }

  set chart_title(val: string) { this.setAttribute('chart_title', (+val as unknown) as string); }

  /** Reflects data attribute for convenience. */
  get data(): string { return this.getAttribute('data')!; }

  set data(val: string) { this.setAttribute('data', (+val as unknown) as string); }

  connectedCallback(): void {
    super.connectedCallback();

    this._upgradeProperty('chart_type');
    this._upgradeProperty('chart_title');
    this._upgradeProperty('data');

    this._render();
  }

  static get observedAttributes(): string[] {
    return ['chart_type', 'chart_title', 'data'];
  }

  attributeChangedCallback(_name: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
    }
  }
}

define('bugs-chart-sk', BugsChartSk);
