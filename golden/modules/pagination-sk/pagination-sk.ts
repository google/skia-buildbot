/**
 * @module module/pagination-sk
 * @description <h2><code>pagination-sk</code></h2>
 *
 * Widget to let user page forward and backward. Page buttons will be
 * disabled/enabled depending on the offset/total/page_size values.
 *
 * Compatible with the server code httputils.PaginationParams.
 *
 * @attr offset {int} indicates the offset into the list of the items we are paged to.
 *
 * @attr page_size {int} the number of items we go forward/backward on a single page.
 *
 * @attr total {int} the total number of items that can be paged through or MANY if the
 *      server doesn't know.
 *
 * @evt page-changed - Sent when user pages forward or backward. Check
 *       e.detail.delta for how many pages changed and which direction.
 */

import { html } from 'lit-html';
import { define } from 'elements-sk/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/styles/buttons';

// MANY (2^31-1, aka math.MaxInt32) is a special value meaning the
// server doesn't know how many items there are, only that it's more
// than are currently being displayed.
const MANY = 2147483647;

export interface PaginationSkPageChangedEventDetail {
  readonly delta: number;
}

export class PaginationSk extends ElementSk {
  private static _template = (ele: PaginationSk) => html`
    <button ?disabled=${ele._currPage() <= 1}
            title="Go to previous page of results."
            @click=${() => ele._page(-1)}
            class="prev">
      Prev
    </button>
    <div class=counter>
      page ${ele._currPage()}
    </div>
    <button ?disabled=${!ele._canGoNext(ele.offset + ele.page_size)}
            title="Go to next page of results."
            @click=${() => ele._page(1)}
            class="next">
      Next
    </button>
    <button ?disabled=${!ele._canGoNext(ele.offset + 5 * ele.page_size)}
            title="Skip forward 5 pages of results."
            @click=${() => ele._page(5)}
            class="skip">
      +5
    </button>
  `;

  constructor() {
    super(PaginationSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('offset');
    this._upgradeProperty('page_size');
    this._upgradeProperty('total');
    this._render();
  }

  static get observedAttributes() {
    return ['offset', 'page_size', 'total'];
  }

  /** Reflects offset attribute for convenience. */
  get offset(): number { return +this.getAttribute('offset')!; }

  set offset(val: number) { this.setAttribute('offset', (+val as unknown) as string); }

  /** Reflects page_size attribute for convenience. */
  get page_size(): number { return +this.getAttribute('page_size')!; }

  set page_size(val: number) { this.setAttribute('page_size', (+val as unknown) as string); }

  /** Reflects total attribute for convenience. */
  get total(): number { return +this.getAttribute('total')!; }

  set total(val: number) { this.setAttribute('total', (+val as unknown) as string); }

  attributeChangedCallback() {
    this._render();
  }

  private _currPage() {
    return Math.round(this.offset / this.page_size) + 1;
  }

  private _canGoNext(next: number) {
    return this.total === MANY ? true : next <= this.total;
  }

  private _page(n: number) {
    this.dispatchEvent(new CustomEvent<PaginationSkPageChangedEventDetail>('page-changed', {
      detail: {
        delta: n,
      },
      bubbles: true,
    }));
  }
}

define('pagination-sk', PaginationSk);
