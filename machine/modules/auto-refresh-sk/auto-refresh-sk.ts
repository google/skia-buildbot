/**
 * @module modules/auto-refresh-sk
 * @description <h2><code>auto-refresh-sk</code></h2>
 *
 * Allows controlling if the current page gets refreshed automatically.
 * Remembers the state of the auto-refresh toggle using window.localStorage.
 *
 * NB - You can't have more than one auto-refresh-sk on the same page, as they
 *    use the same window.localStorage key.
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
  // The identifier of the timeout, or 0 if we are not refreshing.
  private timeout: number = 0;

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
    id=refresh
    @click=${() => ele.toggleRefresh()}
    title="Start/Stop the automatic refreshing of data on the page."
  >
    ${AutoRefreshSk.refreshButtonDisplayValue(ele)}
  </span>`;

  connectedCallback(): void {
    super.connectedCallback();
    // Kick off refreshStep if needed, based on stored value of 'refreshing' in
    // the window.localStore.
    //
    // eslint-disable-next-line no-self-assign
    this.refreshing = this.refreshing;
  }

  toggleRefresh(): void {
    this.refreshing = !this.refreshing;
  }

  private refreshStep(): void {
    if (this.refreshing) {
      this.dispatchEvent(new CustomEvent('refresh-page', { bubbles: true }));
    }
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
    this._render();
    this.refreshStep();
  }
}

define('auto-refresh-sk', AutoRefreshSk);
