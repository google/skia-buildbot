/**
 * @module module/sort-toggle-sk
 * @description <h2><code>sort-toggle-sk</code></h2>
 *
 * "forked" from sort-sk in infra-sk for performance and correctness reasons when the
 * data being sorted changes.
 *
 * sort-toggle-sk renders a sort arrow on the elements marked with data-key and listens to
 * clicks on those elements to change an underlying array. It triggers an event which the client
 * should use to render the many templates, using map or render; whichever is more performant.
 *
 * The keys on data-key will be the fields used to sort the array of objects by.
 *
 * Clients should set data-sort-toggle-sk to be "up" or "down" on the data-key that the data will
 * start off sorted in. After the data is loaded, clients are expected to call sort on this element
 * to make sure the data becomes sorted.
 *
 * @evt sort-changed: The user has changed how to sort the data. The arr passed in via property
 *   is now sorted to match that intent.
 */
import { define } from 'elements-sk/define';
import { $, $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';

export type SortDirection = 'down' | 'up';

// The states to move each button through on a click.
const toggle = (value: string): SortDirection => (value === 'down' ? 'up' : 'down');

export class SortToggleSk<T extends Object> extends ElementSk {
  private _data: Array<T> = [];

  constructor() {
    super(); // There is no template to use for rendering.
  }

  connectedCallback() {
    super.connectedCallback();
    // Attach the icons, but only once.
    $('[data-key]', this).forEach((ele) => {
      // Only attach the icons once.
      if ($$('arrow-drop-down-icon-sk', ele)) {
        return;
      }
      ele.appendChild(document.createElement('arrow-drop-down-icon-sk'));
      ele.appendChild(document.createElement('arrow-drop-up-icon-sk'));
      ele.addEventListener('click', (e) => this._clickHandler(e));
    });
  }

  get data() {
    return this._data;
  }

  set data(d: Array<T>) {
    this._data = d;
  }

  private _setSortAttribute(ele: Element, value: SortDirection) {
    ele.setAttribute('data-sort-toggle-sk', value);
  }

  private _clearSortAttribute(ele: Element) {
    ele.removeAttribute('data-sort-toggle-sk');
  }

  private _getSortAttribute(ele: Element) {
    return ele.getAttribute('data-sort-toggle-sk') || '';
  }

  private _clickHandler(e: Event) {
    let ele = e.target! as HTMLElement;
    // The click might have been on something inside the button (e.g. on the arrow-drop-up-icon-sk),
    // so we want to bubble up to where the key is and set the class that displays the appropriate
    // arrow.
    while (!ele.hasAttribute('data-key') && ele.parentElement !== this) {
      if (ele.parentElement === null) {
        break;
      }
      ele = ele.parentElement;
    }

    if (!ele.dataset.key) {
      throw new DOMException('Inconsistent state: data-key must be non-empty');
    }

    const dir = toggle(this._getSortAttribute(ele));

    $('[data-key]', this).forEach((e) => {
      this._clearSortAttribute(e);
    });
    this._setSortAttribute(ele, dir);

    // Sort the children of the element at #target.
    const sortBy = ele.dataset.key! as keyof T;
    this.sort(sortBy, dir);
  }

  /**
   * Re-sort the data by the given key in the given direction. If alpha is true, it will
   * sort the data as if it were a string (using localeCompare).
   */
  sort(key: keyof T, dir: SortDirection) {
    this._data.sort((a, b) => {
      let left = a[key] as unknown;
      let right = b[key] as unknown;
      if (dir === 'down') {
        [right, left] = [left, right];
      }
      if (typeof left === 'number' && typeof right === 'number') {
        return left - right;
      }
      if (typeof left === 'string' && typeof right === 'string') {
        return left.localeCompare(right);
      }
      throw new Error(
        `Trying to sort by key "${String(key)}", which is neither a number nor a string. ${left}, ${right}`,
      );
    });
    this.dispatchEvent(new CustomEvent('sort-changed', { bubbles: true }));
  }
}

define('sort-toggle-sk', SortToggleSk);
