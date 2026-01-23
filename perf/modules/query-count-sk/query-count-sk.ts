/**
 * @module modules/query-count-sk
 * @description <h2><code>query-count-sk</code></h2>
 *
 * Reports the number of matches for a given query.
 *
 * @attr {string} current_query - The current query to count against.
 *
 * @attr {string} url - The URL to POST the query to.
 *
 * @evt paramset-changed - An event with the updated paramset in e.detail
 *   from the fetch response.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CountHandlerRequest, CountHandlerResponse, ReadOnlyParamSet } from '../json';
import '../../../elements-sk/modules/spinner-sk';

export class QueryCountSk extends ElementSk {
  private _count = 0;

  private _requestInProgress = false;

  private fetchController: AbortController | null = null;

  constructor() {
    super(QueryCountSk.template);
    this._count = 0;
    this._requestInProgress = false;
  }

  private static template = (ele: QueryCountSk) => html`
    <div>
      <span>${ele._count}</span>
      <spinner-sk ?active=${ele._requestInProgress}></spinner-sk>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('url');
    this._upgradeProperty('current_query');
    this._render();
    this._fetch();
  }

  attributeChangedCallback(): void {
    this._fetch();
  }

  static get observedAttributes(): string[] {
    return ['current_query', 'url'];
  }

  private _fetch() {
    if (!this._connected) {
      return;
    }
    if (!this.url || this.current_query === '') {
      return;
    }

    // Force only one fetch at a time. Abort any outstanding requests.
    if (this.fetchController) {
      this.fetchController.abort();
    }
    this.fetchController = new AbortController();
    this._requestInProgress = true;
    const now = Math.floor(Date.now() / 1000);
    const body: CountHandlerRequest = {
      q: this.current_query,
      end: now,
      begin: now - 24 * 60 * 60,
    };
    this._render();
    fetch(this.url, {
      method: 'POST',
      signal: this.fetchController.signal,
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: CountHandlerResponse) => {
        this._count = json.count;
        this._requestInProgress = false;
        this._render();
        this.dispatchEvent(
          new CustomEvent<ReadOnlyParamSet>('paramset-changed', {
            detail: json.paramset,
            bubbles: true,
          })
        );
      })
      .catch((msg) => {
        this._requestInProgress = false;
        if (msg.name === 'AbortError') {
          // User did something to invalidate the request, so just
          // return without updating the UI state or displaying an
          // error message.
          return;
        }
        this._render();
        errorMessage(msg);
      });
  }

  /** @prop url - The URL to make POST requests to.  */
  get url(): string {
    return this.getAttribute('url') || '';
  }

  set url(val: string) {
    this.setAttribute('url', val);
  }

  /** @prop current_query - The current trace query. */
  get current_query(): string {
    return this.getAttribute('current_query') || '';
  }

  set current_query(val: string) {
    this.setAttribute('current_query', val);
    this._count = 0;
    this._render();
  }
}

define('query-count-sk', QueryCountSk);
