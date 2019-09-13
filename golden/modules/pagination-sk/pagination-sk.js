// TODO(kjlubick): add docs and tests

import { html } from 'lit-html'
import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

import 'elements-sk/styles/buttons'

const template = (ele) => html`
<button ?disabled=${ele._currPage() <= 1}
        title="go to previous page of results"
        @click=${() => ele._page(-1)}>
  Prev
</button>
<div>
page ${ele._currPage()}
</div>
<button ?disabled=${!canGoNext(ele.total, ele.offset + ele.page_size)}
         title="go to next page of results"
         @click=${() => ele._page(1)}>
  Next
</button>
<button ?disabled=${!canGoNext(ele.total, ele.offset + 5 * ele.page_size)}
        title="skip forward 5 pages of results"
        @click=${() => ele._page(5)}>
  +5
</button>
`;

function canGoNext(total, next) {
  if (total === 2147483647) {
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

  /** @prop offset {int} indicates the offset into the list of the items we are paged to. */
  get offset() { return +this.getAttribute('offset'); }
  set offset(val) { this.setAttribute('offset', +val); }

  /** @prop page_size {int} indicates the offset into the list of the items we are paged to. */
  get page_size() { return +this.getAttribute('page_size');  }
  set page_size(val) { this.setAttribute('page_size', +val); }

  /** @prop total {int} indicates the offset into the list of the items we are paged to. */
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