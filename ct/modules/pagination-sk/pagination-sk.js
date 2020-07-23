/**
 * @fileoverview A custom element that supports a pagination.
 */

import 'elements-sk/icon/first-page-icon-sk';
import 'elements-sk/icon/last-page-icon-sk';
import 'elements-sk/icon/chevron-left-icon-sk';
import 'elements-sk/icon/chevron-right-icon-sk';
import 'elements-sk/styles/buttons';

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<div>
  <button class=action data-page=0
    ?disabled=${el._onFirstPage()} @click=${el._update}>
    <first-page-icon-sk></first-page-icon-sk>
  </button>
  <button class=action data-page=${el._page - 1}
    ?disabled=${el._onFirstPage()} @click=${el._update}>
    <chevron-left-icon-sk></chevron-left-icon-sk>
  </button>
  ${el._pageButtons.map((page) => html`
    <button data-page=${page}
     @click=${el._update}
     ?disabled=${page === el._page}>${page + 1}</button>`)}
  <button class=action data-page=${el._page + 1}
    ?disabled=${el._onLastPage()} @click=${el._update}>
    <chevron-right-icon-sk></chevron-right-icon-sk>
  </button>
  <button class=action data-page=${el._allPages - 1}
    ?disabled=${el._onLastPage()} @click=${el._update}>
    <last-page-icon-sk></last-page-icon-sk>(${el._allPages})
  </button>
</div>
`;

define('pagination-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._upgradeProperty('pagination');
    this._upgradeProperty('showPages');
    this._showPages = this._showPages || 5;
    this._pagination = this._pagination || { size: 10, offset: 0, total: 0 };
    this._showPagesOffset = Math.floor(this._showPages / 2);
    this._computePageButtons();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _computePageButtons() {
    this._pageButtons = [];
    this._allPages = Math.ceil(this._pagination.total / this._pagination.size);
    this._showPagesOffset = Math.floor(this._showPages / 2);

    this._page = Math.floor(this._pagination.offset / this._pagination.size);
    const start = Math.max(Math.min(this._page - this._showPagesOffset,
      this._allPages - this._showPages), 0);
    const end = Math.min(start + this._showPages - 1, this._allPages - 1);
    for (let i = start; i <= end; i++) {
      this._pageButtons.push(i);
    }
    this._render();
  }

  _update(e) {
    const targetPage = e.currentTarget.dataset.page;
    this._pagination.offset = targetPage * this.pagination.size;
    this._computePageButtons();
    this.dispatchEvent(new CustomEvent('page-changed',
      { bubbles: true, detail: { offset: this._pagination.offset } }));
  }

  _onFirstPage() {
    return this._page === 0;
  }

  _onLastPage() {
    return this._allPages === 0 || this._page === (this._allPages - 1);
  }

  /**
   * @prop {Object} pagination - Pagination data {offset, size, total}.
   */
  get pagination() {
    return this._pagination;
  }

  set pagination(val) {
    this._pagination = val;
    this._computePageButtons();
  }

  /**
   * @prop {Number} showPages - Number of page buttons to display, centered
   * around the current page.
   */
  get showPages() {
    return this._showPages;
  }

  set showPages(val) {
    this._showPages = val;
    this._computePageButtons();
  }
});
