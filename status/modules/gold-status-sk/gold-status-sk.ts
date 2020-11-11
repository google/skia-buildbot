/**
 * @module modules/gold-status-sk
 * @description <h2><code>gold-status-sk</code></h2>
 *
 * Custom element to display untriaged Gold iamges.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { StatusResponse } from '../../../golden/modules/rpc_types';

const goldUrl = 'https://gold.skia.org';

export class GoldStatusSk extends ElementSk {
  private resp?: StatusResponse;
  private static template = (el: GoldStatusSk) => html`
    <div class="table">
      ${el.resp && el.resp.corpStatus
        ? el.resp!.corpStatus!.map(
            (c) => html`
              <a
                class="tr"
                href="${goldUrl}${`/?corpus=${c!.name}`}"
                target="_blank"
                rel="noopener noreferrer"
                title="Skia Gold: Untriaged ${c!.name} image count"
              >
                <div class="td">${c!.name}</div>
                <div class="td number"><span class="value">${c!.untriagedCount}</span></div>
              </a>
            `
          )
        : html``}
    </div>
  `;

  constructor() {
    super(GoldStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    fetch(`${goldUrl}/json/v1/trstatus`, { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: StatusResponse) => {
        this.resp = json;
        this.resp.corpStatus?.sort((a, b) => b!.untriagedCount - a!.untriagedCount);
        this._render();
      })
      .catch(errorMessage)
      .finally(() => {
        window.setTimeout(() => this.refresh(), 60 * 1000);
      });
  }
}

define('gold-status-sk', GoldStatusSk);
