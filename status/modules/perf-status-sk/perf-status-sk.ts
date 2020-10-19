/**
 * @module modules/perf-status-sk
 * @description <h2><code>perf-status-sk</code></h2>
 *
 * Custom element for displaying status of Perf regressions.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { AlertsStatus } from '../../../perf/modules/json';

export class PerfStatusSk extends ElementSk {
  private resp: AlertsStatus = { alerts: 0 };
  private static template = (el: PerfStatusSk) => html`
    <div class="table">
      <a
        class="tr"
        href="https://perf.skia.org/t/?filter=cat%3AProd"
        target="_blank"
        rel="noopener noreferrer"
        title="Active Perf Alerts"
      >
        <div class="td">regressions</div>
        <div class="td number"><span class="value">${el.resp.alerts}</span></div>
      </a>
    </div>
  `;

  constructor() {
    super(PerfStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    fetch('https://perf.skia.org/_/alerts/', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: AlertsStatus) => {
        this.resp = json;
        this._render();
      })
      .finally(() => {
        window.setTimeout(() => this.refresh(), 60 * 1000);
      });
  }
}

define('perf-status-sk', PerfStatusSk);
