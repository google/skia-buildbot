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

import { html } from 'lit-html'
import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

import 'elements-sk/styles/buttons'

const template = (ele) => html`
<button ?disabled=${ele._currPage() <= 1}
        title="Go to previous page of results."
        @click=${() => ele._page(-1)}
        class="prev">
  Prev
</button>
<div class=counter>
  page ${ele._currPage()}
</div>
<button ?disabled=${!canGoNext(ele.total, ele.offset + ele.page_size)}
         title="Go to next page of results."
         @click=${() => ele._page(1)}
         class="next">
  Next
</button>
<button ?disabled=${!canGoNext(ele.total, ele.offset + 5 * ele.page_size)}
        title="Skip forward 5 pages of results."
        @click=${() => ele._page(5)}
        class="skip">
  +5
</button>
`;

// MANY (2^31-1, aka math.MaxInt32) is a special value meaning the
// server doesn't know how many items there are, only that it's more
// than are currently being displayed.
const MANY = 2147483647

function canGoNext(total, next) {
  if (total === MANY) {
    return true;
  }
  return next <= total;
}

define('pagination-sk', class extends ElementSk {
  constructor() {
    super(template);
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

  /** @prop offset {int} Reflects offset attribute for convenience. */
  get offset() { return +this.getAttribute('offset'); }
  set offset(val) { this.setAttribute('offset', +val); }

  /** @prop page_size {int} Reflects page_size attribute for convenience. */
  get page_size() { return +this.getAttribute('page_size');  }
  set page_size(val) { this.setAttribute('page_size', +val); }

  /** @prop total {int} Reflects total attribute for convenience. */
  get total() { return +this.getAttribute('total'); }
  set total(val) {  this.setAttribute('total', +val);  }

  attributeChangedCallback() {
    this._render();
  }

  _currPage() {
    return Math.round(this.offset/this.page_size) + 1;
  }

  _page(n) {
    this.dispatchEvent(new CustomEvent('page-changed', { detail: {
      delta: n,
    }, bubbles: true}));
  }
});
