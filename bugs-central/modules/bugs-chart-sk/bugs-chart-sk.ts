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
 * @attr client {string} The name of the client. Eg: Android/Chromium/Flutter/Skia.
 *
 * @attr source {string} The name of the issue source. Eg: Github/Monorail.
 *
 * @attr query {string} The name of the query. Eg: is:open.
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
      data="/_/get_chart_data?client=${el.client}&source=${el.source}&query=${encodeURIComponent(el.query)}&type=${el.chart_type}">
    </google-chart>
`;

  /** Reflects chart_type attribute for convenience. */
  get chart_type(): string { return this.getAttribute('chart_type')!; }

  set chart_type(val: string) { this.setAttribute('chart_type', (+val as unknown) as string); }

  /** Reflects chart_type attribute for convenience. */
  get chart_title(): string { return this.getAttribute('chart_title')!; }

  set chart_title(val: string) { this.setAttribute('chart_title', (+val as unknown) as string); }

  /** Reflects client attribute for convenience. */
  get client(): string { return this.getAttribute('client')!; }

  set client(val: string) { this.setAttribute('client', (+val as unknown) as string); }

  /** Reflects source attribute for convenience. */
  get source(): string { return this.getAttribute('source')!; }

  set source(val: string) { this.setAttribute('source', (+val as unknown) as string); }

  /** Reflects query attribute for convenience. */
  get query(): string { return this.getAttribute('query')!; }

  set query(val: string) { this.setAttribute('query', (+val as unknown) as string); }

  connectedCallback(): void {
    super.connectedCallback();

    this._upgradeProperty('chart_type');
    this._upgradeProperty('chart_title');
    this._upgradeProperty('client');
    this._upgradeProperty('source');
    this._upgradeProperty('query');

    this._render();
  }

  static get observedAttributes(): string[] {
    return ['chart_type', 'chart_title', 'client', 'source', 'query'];
  }

  attributeChangedCallback(_name: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
    }
  }
}

define('bugs-chart-sk', BugsChartSk);
