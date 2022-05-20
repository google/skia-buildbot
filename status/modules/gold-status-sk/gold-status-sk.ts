/**
 * @module modules/gold-status-sk
 * @description <h2><code>gold-status-sk</code></h2>
 *
 * @property repo: string - The repository we are currently looking at.

 * Custom element to display untriaged Gold iamges.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { GUICorpusStatus, StatusResponse } from '../../../golden/modules/rpc_types';

const defaultGoldURL = 'https://gold.skia.org';

// The Gold URLs of Skia repos are hardcoded because there is no way
// to compute them based on the repo names.
const repoToGoldURL: Record<string, string> = {
  skia: defaultGoldURL,
  infra: 'https://skia-infra-gold.skia.org',
  eskia: 'https://eskia-gold.skia.org',
  lottie: 'https://lottie-gold.skia.org',
};

export class GoldStatusSk extends ElementSk {
  private resp?: StatusResponse;

  private _repo: string = '';

  private static template = (el: GoldStatusSk) => html`
    <div class="table">
      ${el.resp && el.resp.corpStatus
    ? el.resp!.corpStatus!.map(
      (c) => html`
              <a
                class="tr"
                href="${el.getGoldURL()}${`/?corpus=${c!.name}`}"
                target="_blank"
                rel="noopener noreferrer"
                title="Skia Gold: Untriaged ${c!.name} image count"
              >
                <div class="td">${c!.name}</div>
                <div class="td number">
                  <span class="value">${c!.untriagedCount}</span>
                </div>
              </a>
            `,
    )
    : html``}
    </div>
  `;

  constructor() {
    super(GoldStatusSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  get repo(): string {
    return this._repo;
  }

  set repo(v: string) {
    this._repo = v.toLowerCase();
    this._render();
    this.refresh();
  }

  private getGoldURL(): string {
    if (repoToGoldURL[this.repo]) {
      return repoToGoldURL[this.repo];
    }
    return defaultGoldURL;
  }

  private refresh() {
    const goldUrl = this.getGoldURL();
    fetch(`${goldUrl}/json/v2/trstatus`, { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: StatusResponse) => {
        this.resp = json;
        this.resp.corpStatus?.sort(
          (a: GUICorpusStatus, b: GUICorpusStatus) => b!.untriagedCount - a!.untriagedCount,
        );
        this._render();
      })
      .catch(errorMessage)
      .finally(() => {
        window.setTimeout(() => this.refresh(), 60 * 1000);
      });
  }
}

define('gold-status-sk', GoldStatusSk);
