/**
 * @module modules/sort-sk
 * @description <h2><code>sort-sk</code></h2>
 *
 * Allows sorting the members of the indicated element by the values of the
 * data attributes.
 *
 * This element does *not* support the elements to sort changing. It assumes the data in
 * the table is constant (otherwise, lit-html will not be able to render it accurately).
 *
 * Add children to <sort-sk> that generate click events and that have child
 * content, such as buttons. Add a data-key * attribute to each child element
 * that indicates which data-* attribute the children should be sorted on.
 *
 * Note that all sorting is done numerically, unless the
 * 'data-sort-type=alpha' attribute is set on the element generating the
 * click, in which case the sorting is done alphabetically.
 *
 * Additionally a single child element can have a data-default attribute with
 * a value of 'up' or 'down' to indicate the default sorting that already
 * exists in the data.
 *
 *
 * @example An example usage, that will present two buttons to sort the contents of
 *   div#stuffToBeSorted.
 *
 *    <sort-sk target=stuffToBeSorted>
 *      <button data-key=clustersize data-default=down>Cluster Size </button>
 *      <button data-key=stepsize data-sort-type=alpha>Name</button>
 *    </sort-sk>
 *
 *    <div id=stuffToBeSorted>
 *      <div data-clustersize=10 data-name=foo></div>
 *      <div data-clustersize=50 data-name=bar></div>
 *      ...
 *    </div>
 *
 * @attr target - The id of the container element whose children are to be sorted.
 *
 */
import { define } from 'elements-sk/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $, $$ } from 'common-sk/modules/dom';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';

export type SortDirection = 'down' | 'up';

// The states to move each button through on a click.
const toggle = (value: string): SortDirection => {
  return value === 'down' ? 'up' : 'down';
};

interface SortableEntry {
  value: string;
  valueAsNumber: number;
  node: HTMLElement;
}

// Functions to pass to sort().
const f_alpha_up = (x: SortableEntry, y: SortableEntry) => {
  return x.value.localeCompare(y.value);
};
const f_alpha_down = (x: SortableEntry, y: SortableEntry) => f_alpha_up(y, x);
const f_num_up = (x: SortableEntry, y: SortableEntry) => {
  if (x.valueAsNumber === y.valueAsNumber) {
    return 0;
  } else if (x.valueAsNumber > y.valueAsNumber) {
    return 1;
  } else {
    return -1;
  }
};
const f_num_down = (x: SortableEntry, y: SortableEntry) => f_num_up(y, x);

export class SortSk extends ElementSk {
  connectedCallback() {
    super.connectedCallback();
    $('[data-key]', this).forEach((ele) => {
      // Only attach the icons once.
      if (ele.querySelector('arrow-drop-down-icon-sk')) {
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
    }
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
   * sort the data as if it were a string (using localeCompare).
   */
  sort(key: string, dir: SortDirection, alpha: boolean) {
    // Remember the direction we are sorting in.
    const up = dir === 'up';

    const container = this.parentElement!.querySelector(
        `#${this.getAttribute('target')}`
    );
    if (container === null) {
      throw 'Failed to find "target" attribute.';
    }
    const arr: SortableEntry[] = [];
    for (const ele of Array.from(container.children)) {
      const htmlEle = ele as HTMLElement;
      const value: string = htmlEle.dataset[key] || '';
      const entry = {
        value: value,
        valueAsNumber: +value,
        node: htmlEle,
      };
      arr.push(entry);
    }

    // Pick the desired sort function.
    let f = f_alpha_up;
    if (alpha) {
      f = up ? f_alpha_up : f_alpha_down;
    } else {
      f = up ? f_num_up : f_num_down;
    }
    arr.sort(f);

    // Rearrange the elements in the sorted order.
    arr.forEach((e) => {
      // Reminder: appendChild will *move* existing nodes, which is what we want here while
      // reordering things.
      container!.appendChild(e.node);
    });
  }
}

define('sort-sk', SortSk);
