/**
 * @module modules/auto-refresh-sk
 * @description <h2><code>auto-refresh-sk</code></h2>
 *
 * Allows controlling if the current page gets refreshed automatically.
 * Remembers the state of the auto-refresh toggle using window.localStorage.
 *
 * @evt refresh-page - This event bubbles, and is produced every time the data
 *   on the page should be refreshed.
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';

const REFRESH_LOCALSTORAGE_KEY = 'autorefresh';

const REFRESH_DURATION_MS = 2000;

export class AutoRefreshSk extends ElementSk {
  timeout: number = 0;

  constructor() {
    super(AutoRefreshSk.template);
  }

  private static refreshButtonDisplayValue = (ele: AutoRefreshSk): TemplateResult => {
    if (ele.refreshing) {
      return html`
        <pause-icon-sk></pause-icon-sk>
      `;
    }
    return html`
      <play-arrow-icon-sk></play-arrow-icon-sk>
    `;
  };

  private static template = (ele: AutoRefreshSk) => html`
  <span
    @click=${() => ele.toggleRefresh()}
    title="Start/Stop the automatic refreshing of data on the page."
  >
    ${AutoRefreshSk.refreshButtonDisplayValue(ele)}
  </span>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  toggleRefresh(): void {
    this.refreshing = !this.refreshing;
    this._render();
    this.refreshStep();
  }

  private async refreshStep(): Promise<void> {
    this.dispatchEvent(new CustomEvent('refresh-page', { bubbles: true }));
    if (this.refreshing && this.timeout === 0) {
      this.timeout = window.setTimeout(() => {
        // Only done here, so multiple calls to refreshStep() won't start
        // a parallel setTimeout chain.
        this.timeout = 0;

        this.refreshStep();
      }, REFRESH_DURATION_MS);
    }
  }

  /** True if the data on the page is periodically refreshed. */
  get refreshing(): boolean {
    return window.localStorage.getItem(REFRESH_LOCALSTORAGE_KEY) === 'true';
  }

  set refreshing(val: boolean) {
    window.localStorage.setItem(REFRESH_LOCALSTORAGE_KEY, `${!!val}`);
  }
}

define('auto-refresh-sk', AutoRefreshSk);
