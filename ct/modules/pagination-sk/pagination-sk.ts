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
import {
  ResponsePagination,
} from '../json';

export class PaginationSk extends ElementSk {
  private _showPages: number = 5;

  private _pagination: ResponsePagination = { size: 10, offset: 0, total: 0 };

  private _showPagesOffset = Math.floor(this._showPages / 2);

  private _pageButtons: number[] = [];

  private _page: number = 0;

  private _allPages: number = 0;

  constructor() {
    super(PaginationSk.template);
    this._upgradeProperty('pagination');
    this._upgradeProperty('showPages');
    this._computePageButtons();
  }

  private static template = (el: PaginationSk) => html`
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

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  _computePageButtons(): void {
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

  _update(e: Event): void {
    const targetPage = (e.currentTarget! as HTMLElement).dataset.page || '0';
    this._pagination.offset = parseFloat(targetPage) * this.pagination.size;
    this._computePageButtons();
    this.dispatchEvent(new CustomEvent('page-changed',
      { bubbles: true, detail: { offset: this._pagination.offset } }));
  }

  _onFirstPage(): boolean {
    return this._page === 0;
  }

  _onLastPage(): boolean {
    return this._allPages === 0 || this._page === (this._allPages - 1);
  }

  /**
   * @prop {Object} pagination - Pagination data {offset, size, total}.
   */
  get pagination(): ResponsePagination {
    return this._pagination;
  }

  set pagination(val: ResponsePagination) {
    this._pagination = val;
    this._computePageButtons();
  }

  /**
   * @prop {Number} showPages - Number of page buttons to display, centered
   * around the current page.
   */
  get showPages(): number {
    return this._showPages;
  }

  set showPages(val: number) {
    this._showPages = val;
    this._computePageButtons();
  }
}

define('pagination-sk', PaginationSk);
