/**
 * @module module/paramset-sk
 * @description <h2><code>paramset-sk</code></h2>
 *
 * The paramset-sk element displays a paramset and generates events
 * as the params and labels are clicked.
 *
 * @evt paramset-key-click - Generated when the key for a paramset is clicked.
 *     The name of the key will be sent in e.detail.key. The value of
 *     e.detail.ctrl is true if the control key was pressed when clicking.
 *
 *      {
 *        key: "arch",
 *        ctrl: false,
 *      }
 *
 * @evt paramset-key-value-click - Generated when one value for a paramset is clicked.
 *     The name of the key will be sent in e.detail.key, the value in
 *     e.detail.value. The value of e.detail.ctrl is true if the control key
 *     was pressed when clicking.
 *
 *      {
 *        key: "arch",
 *        value: "x86",
 *        ctrl: false,
 *      }
 *
 * @attr {string} clickable - If true then keys and values look like they are clickable
 *     i.e. via color, text-decoration, and cursor. If clickable is false
 *     then this element won't generate the events listed below, and the
 *     keys and values are not styled to look clickable. Setting both
 *     clickable and clickable_values is unsupported.
 *
 * @attr {string} clickable_values - If true then only the values are clickable. Setting
 *     both clickable and clickable_values is unsupported.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';

const _paramsetValue = (ele, key, params) => params.map((value) => html`<div class=${ele._highlighted(key, value)} data-key=${key} data-value=${value}>${value}</div>`);

const _paramsetValues = (ele, key) => ele._paramsets.map((p) => html`<td>
  ${_paramsetValue(ele, key, p[key] || [])}
</td>`);

const _row = (ele, key) => html`
  <tr>
    <th data-key=${key}>${key}</th>
    ${_paramsetValues(ele, key)}
  </tr>`;

const _rows = (ele) => ele._sortedKeys.map((key) => _row(ele, key));

const _titles = (ele) => ele._normalizedTitles().map((t) => html`<th>${t}</th>`);

const template = (ele) => html`
  <table @click=${ele._click} class=${ele._computeClass()}>
    <tbody>
      <tr>
        <th></th>
        ${_titles(ele)}
      </tr>
      ${_rows(ele)}
    </tbody>
  </table>
`;

define('paramset-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._titles = [];
    this._paramsets = [];
    this._sortedKeys = [];
    this._highlight = {};
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('paramsets');
    this._upgradeProperty('highlight');
    this._upgradeProperty('clickable');
    this._upgradeProperty('clickable_values');
    this._render();
  }

  _computeClass() {
    if (this.clickable_values) {
      return 'clickable_values';
    } if (this.clickable) {
      return 'clickable';
    }
    return '';
  }

  _highlighted(key, value) {
    return this._highlight[key] === value ? 'highlight' : '';
  }

  _click(e) {
    if (!this.clickable && !this.clickable_values) {
      return;
    }
    const t = e.target;
    if (!t.dataset.key) {
      return;
    }
    if (t.nodeName === 'TH') {
      if (!this.clickable) {
        return;
      }
      const detail = {
        key: t.dataset.key,
        ctrl: e.ctrlKey,
      };
      this.dispatchEvent(new CustomEvent('paramset-key-click', {
        detail,
        bubbles: true,
      }));
    } else if (t.nodeName === 'DIV') {
      const detail = {
        key: t.dataset.key,
        value: t.dataset.value,
        ctrl: e.ctrlKey,
      };
      this.dispatchEvent(new CustomEvent('paramset-key-value-click', {
        detail,
        bubbles: true,
      }));
    }
  }

  static get observedAttributes() {
    return ['clickable', 'clickable_values'];
  }

  /** @prop clickable {string} Mirrors the clickable attribute.  */
  get clickable() { return this.hasAttribute('clickable'); }

  set clickable(val) {
    if (val) {
      this.setAttribute('clickable', '');
    } else {
      this.removeAttribute('clickable');
    }
  }

  /** @prop clickable_values {string} Mirrors the clickable_values attribute.  */
  get clickable_values() { return this.hasAttribute('clickable_values'); }

  set clickable_values(val) {
    if (val) {
      this.setAttribute('clickable_values', '');
    } else {
      this.removeAttribute('clickable_values');
    }
  }

  attributeChangedCallback() {
    this._render();
  }

  /** @prop titles {Array<String>} Array that represents the title of the columns. */
  get titles() { return this._titles; }

  set titles(val) {
    this._titles = val;
    this._render();
  }

  // Returns the titles specified by the user, or an empty title for each paramset
  // if the number of specified titles and the number of paramsets don't match.
  _normalizedTitles() {
    if (this._titles.length === this._paramsets.length) {
      return this._titles;
    }
    return new Array(this._paramsets.length).fill('');
  }

  /** @prop paramsets {Object} An array of serialized paramtools.ParamSets. */
  get paramsets() { return this._paramsets; }

  set paramsets(val) {
    this._paramsets = val;

    // Compute a rolled up set of all parameter keys across all paramsets.
    const allKeys = {};
    this._paramsets.forEach((p) => {
      Object.keys(p).forEach((key) => {
        allKeys[key] = true;
      });
    });
    this._sortedKeys = Object.keys(allKeys);
    this._sortedKeys.sort();
    this._render();
  }

  /** @prop highlight {Object} A serialized paramtools.Params.  */
  get highlight() { return this._highlight; }

  set highlight(val) {
    this._highlight = val;
    this._render();
  }
});
