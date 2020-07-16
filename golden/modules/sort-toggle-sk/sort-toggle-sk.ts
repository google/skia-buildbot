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
const toggle = (value: string): SortDirection => {
  return value === 'down' ? 'up' : 'down';
};

export class SortToggleSk extends ElementSk {

  private _data: Array<Object>;

  constructor() {
    super(null); // There is no template to use for rendering.
    this._data = [];
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

    // Handle a default value if one has been set.
    const def = $$<HTMLElement>('[data-default]', this);
    if (def) {
      this._setSortClass(def, def.dataset.default! as SortDirection);
      // Sort on the next frame to avoid infinite callbacks where sorting triggers an event
      // which triggers rendering, etc.
      requestAnimationFrame(() => {
        this.sort(def.dataset.key!, def.dataset.default! as SortDirection,
            def.dataset.sortType === 'alpha');
      });
    }
  }

  get data() {
    return this._data;
  }

  set data(d: Array<Object>) {
    this._data = d;
    this._render();
  }

  _setSortClass(ele: Element, value: SortDirection) {
    ele.setAttribute('data-sort-sk', value);
  }

  _clearSortClass(ele: Element) {
    ele.removeAttribute('data-sort-sk');
  }

  _getSortClass(ele: Element) {
    return ele.getAttribute('data-sort-sk') || '';
  }

  _clickHandler(e: Event) {
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

    const dir = toggle(this._getSortClass(ele));

    $('[data-key]', this).forEach((e) => {
      this._clearSortClass(e);
    });
    this._setSortClass(ele, dir);

    // Are we sorting alphabetically or numerically.
    const alpha = ele.dataset.sortType === 'alpha';

    // Sort the children of the element at #target.
    const sortBy = ele.dataset.key || '(key not found)';
    this.sort(sortBy, dir, alpha);
  }

  /** Re-sort the data by the given key in the given direction. If alpha is true, it will
   * sort the data as if it were a string (using localeCompare). */
  sort(key: string, dir: SortDirection, alpha: boolean) {
    // Remember the direction we are sorting in.
    const up = dir === 'up';

    // sort the data appropriately.
    if (alpha) {
      if (up) {
        this._data.sort((a: Object, b: Object) => {
          // @ts-ignore
          return a[key].localeCompare(b[key]);
        });
      } else {
        this._data.sort((a: Object, b: Object) => {
          // @ts-ignore
          return b[key].localeCompare(a[key]);
        });
      }
    } else {
      // numeric sort
      if (up) {
        this._data.sort((a: Object, b: Object) => {
          // @ts-ignore
          return a[key] - b[key];
        });
      } else {
        this._data.sort((a: Object, b: Object) => {
          // @ts-ignore
          return b[key] - a[key];
        });
      }
    }
    this.dispatchEvent(new CustomEvent('sort-changed', {bubbles: true}));
  }
}

define('sort-toggle-sk', SortToggleSk);
