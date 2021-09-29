/**
 * @module modules/bugs-status-sk
 * @description <h2><code>bugs-status-sk</code></h2>
 *
 * Custom element to display untriaged Skia bugs.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, TemplateResult } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { GetClientCountsResponse } from '../../../bugs-central/modules/json';

const bugsCentralUrl = 'https://bugs-central.skia.org';

export class BugsStatusSk extends ElementSk {
  private resp?: GetClientCountsResponse;

  constructor() {
    super(BugsStatusSk.template);
  }

  private static template = (el: BugsStatusSk) => html`
    <div class="table">
      ${el.resp && el.resp.clients_to_status_data
    ? el.displayBugsData()
    : html``}
    </div>
  `;

  displayBugsData(): TemplateResult[] {
    const rows: TemplateResult[] = [];
    Object.keys(this.resp!.clients_to_status_data!).forEach((client: string) => rows.push(html`
        <a
          class="tr"
          href="${this.resp!.clients_to_status_data![client].link}"
          target="_blank"
          rel="noopener noreferrer"
          title="Untriaged ${client!} bugs count"
        >
          <div class="td">${client!}</div>
          <div class="td number">
            <span class="value"
              >${this.resp!.clients_to_status_data![client]
  .untriaged_count}</span
            >
          </div>
        </a>
      `));
    return rows;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    fetch(`${bugsCentralUrl}/get_client_counts`, { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: GetClientCountsResponse) => {
        this.resp = json;
        this._render();
      })
      .catch(errorMessage)
      .finally(() => {
        window.setTimeout(() => this.refresh(), 60 * 1000);
      });
  }
}

define('bugs-status-sk', BugsStatusSk);
