/**
 * @module module/sort-sk
 * @description <h2><code>sort-sk</code></h2>
 *
 * Allows sorting the members of the indicated element by the values of the
 * data attributes.
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
import { define } from 'elements-sk/define'
import { html } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { $, $$ } from 'common-sk/modules/dom'
import 'elements-sk/icon/arrow-drop-down-icon-sk'
import 'elements-sk/icon/arrow-drop-up-icon-sk'

// The states to move each button through on a click.
const toggle = {
  '': 'down',
  'down': 'up',
  'up': 'down',
};

// Functions to pass to sort().
const f_alpha_up = (x, y) => {
  if (x.value === y.value) {
    return 0;
  }
  return  x.value > y.value ? 1 : -1;
};
const f_alpha_down = (x, y) => f_alpha_up(y, x);
const f_num_up = (x, y) => (x.value - y.value);
const f_num_down = (x, y) => f_num_up(y, x);

define('sort-sk', class extends ElementSk {
  connectedCallback() {
    super.connectedCallback();
    $('[data-key]', this).forEach((ele) => {
      // Only attach the icons once.
      if (ele.querySelector('arrow-drop-down-icon-sk')) {
        return
      }
      ele.appendChild(document.createElement('arrow-drop-down-icon-sk'));
      ele.appendChild(document.createElement('arrow-drop-up-icon-sk'));
      ele.addEventListener('click', (e) => this._clickHandler(e));
    });

    // Handle a default value if one has been set.
    const def = $$('[data-default]', this);
    if (def) {
      this._setSortClass(def, def.dataset.default);
    }
  }

  _setSortClass(ele, value) {
    ele.setAttribute('data-sort-sk', value);
  }

  _clearSortClass(ele) {
    ele.removeAttribute('data-sort-sk');
  }

  _getSortClass(ele) {
    return ele.getAttribute('data-sort-sk') || '';
  }

  _clickHandler(e) {
    let ele = e.target;
    while (ele.parentNode !== this) {
      ele = ele.parentNode;
    }

    const dir = toggle[this._getSortClass(ele)];

    $('[data-key]', this).forEach((e) => {
      this._clearSortClass(e);
    });
    this._setSortClass(ele, dir);

    // Remember the direction we are sorting in.
    let up = dir === 'up';

    // Are we sorting alphabetically or numerically.
    let alpha = ele.dataset.sortType === 'alpha';

    // Sort the children of the element at #target.
    let sortBy = ele.dataset.key;
    let container = this.parentElement.querySelector(`#${this.getAttribute('target')}`);
    let arr = [];
    for (const ele of container.children) {
      let value = ele.dataset[sortBy];
      if (!alpha) {
        value = +value;
      }
      arr.push({
        value: value,
        node: ele
      });
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
      container.appendChild(e.node);
    });
  }
});

