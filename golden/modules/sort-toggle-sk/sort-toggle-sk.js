/**
 * @module module/sort-toggle-sk
 * @description <h2><code>sort-toggle-sk</code></h2>
 *
 * A sort-toggle is a set of linked indicators that can be clicked to allow a table to be sorted
 * by a given column in either ascending or descending order.
 *
 * It is based on sort-toggle from Swarming, released under the Apache License.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';

const template = (ele) => html`
<div @click=${ele.toggle}>
  <arrow-drop-down-icon-sk ?hidden=${ele.key === ele.currentKey && ele.direction === 'asc'}>
  </arrow-drop-down-icon-sk>
  <arrow-drop-up-icon-sk ?hidden=${ele.key === ele.currentKey && ele.direction === 'desc'}>
  </arrow-drop-up-icon-sk>
</div>`;

define('sort-toggle-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._currentKey = '';
    this._key = '';
    this._direction = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop {string} currentKey - The currently selected sort key for a
   *                  group of sort-toggles. This should be set if a
   *                  sort-changed event from another sort-toggle was
   *                  observed.
   */
  get currentKey() {
    return this._currentKey;
  }

  set currentKey(val) {
    this._currentKey = val;
    this._render();
  }

  /** @prop {string} key - An arbitrary, unique string that this sort-toggle
   *                  represents.
   */
  get key() {
    return this._key;
  }

  set key(val) {
    this._key = val;
    this._render();
  }

  /** @prop {string} direction - Either 'asc' or 'desc' indicating which
   *                  direction the user indicated. Is ignored if currentKey
   *                  does not equal this.key.
   */
  get direction() {
    return this._direction;
  }

  set direction(val) {
    this._direction = val;
    this._render();
  }

  toggle() {
    if (this.currentKey === this.key) {
      if (this.direction === 'asc') {
        this.direction = 'desc';
      } else {
        this.direction = 'asc';
      }
    } else {
      // Force ascending when we switch what is being sorted by.
      this.direction = 'asc';
    }
    // Set this toggle to be active
    this.currentKey = this.key;

    /**
     * Sort change event - a user has indicated the sort direction
     * should be changed.
     *
     * @event sort-change
     * @type {object}
     * @property {string} direction - 'asc' or 'desc' for
     *                    ascending/descending
     * @property {string} key - The key of the toggle that was clicked.
     */
    this.dispatchEvent(new CustomEvent('sort-change', {
      detail: {
        direction: this.direction,
        key: this.key,
      },
      bubbles: true,
    }));
  }
});
