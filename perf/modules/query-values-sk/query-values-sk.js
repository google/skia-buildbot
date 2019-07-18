/**
 * @module modules/query-values-sk
 * @description <h2><code>query-values-sk</code></h2>
 *
 * The right-hand side of the query-sk element, the values for a single key
 * in a query/paramset.
 *
 * @evt query-values-changed - Trigggered only when the selections have actually
 *     changed. The selection is available in e.detail.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import 'elements-sk/checkbox-sk'
import 'elements-sk/multi-select-sk'


const values = (ele) => {
  return ele._options.map((v) => html`
    <div value=${v} ?selected=${ele._selected.indexOf(v) != -1 || ele._selected.indexOf(v.slice(1)) != -1}>${v}</div>
  `);
};

const template = (ele) => html`
  <checkbox-sk id=invert @change=${ele._invertChange} title='Match items not selected below.' label='Invert'> </checkbox-sk>
  <checkbox-sk id=regex @change=${ele._regexChange} title='Match items via regular expression.' label='Regex'> </checkbox-sk>
  <input type=text id=regexValue class=hidden @input=${ele._selectionChange}>
  <multi-select-sk
    id=values
    @selection-changed=${ele._selectionChange}>
    ${values(ele)}
  </multi-select-sk>
  `;

window.customElements.define('query-values-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._options = [];
    this._selected = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._invert = this.querySelector('#invert');
    this._regex = this.querySelector('#regex');
    this._regexValue = this.querySelector('#regexValue');
    this._upgradeProperty('options');
    this._upgradeProperty('selected');
  }

  _invertChange(e) {}
  _regexChange(e) {}

  _selectionChange(e) {
    this._updateModel();
    //this._fireEvent();
  }

  /** @prop options {Array} The available options as an Array of strings. */
  get options() { return this._options }
  set options(val) {
    this._options = val;
    this._updateModel();
  }

  /** @prop selected {Array} Current selections. */
  get selected() { return this._selected }
  set selected(val) {
    this._selected = val;
    this._updateModel();
  }

  _updateModel() {
    this._invert.checked = !!(this._selected.length >= 1 && this._selected[0][0] === "!");
    this._regex.checked = !!(this._selected.length == 1 && this._selected[0][0] === "~");
    if (this._regex.checked) {
      this._regexValue.value = this._regexDisplayValue(this._selected);
    }
    this._render()
  }

  _regexDisplayValue(arr) {
    if (arr && arr.length > 0) {
      return arr[0].slice(1);
    }
    return '';
  }

});
