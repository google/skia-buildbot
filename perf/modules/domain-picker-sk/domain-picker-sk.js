/**
 * @module modules/domain-picker-sk
 * @description <h2><code>domain-picker-sk</code></h2>
 *
 * Allows picking either a date range for commits, or for
 * picking a number of commits to show before a selected
 * date.
 *
 * @evt
 *
 * @attr domain-changed - The event detail will contain the updated 'state'.
 *
 * @example
 */
import { html, render } from 'lit-html'
import { $ } from 'common-sk/modules/dom'
import 'elements-sk/dialog-sk'
import 'elements-sk/radio-sk'

const _description = (ele) {
  const begin = new Date(ele._state.begin*1000);
  const end = new Date(ele._state.end*1000);
  if (ele._state.request_type === 0) {
    return `${begin.toLocaleDateString()} - ${end.toLocaleDateString()}`;
  } else {
    return `${ele._state.num_commits} commits ending at ${end.toLocaleDateString()}`;
  }
}

const _toDate = (seconds) => {
  return new Date(seconds*1000);
};

const _request_type = (ele) => {
 if (ele._state.request_type === 0) {
   return html`
     <p>Display all points in the date range.</p>
     <label>
       Begin:
       <elix-date-combo-box @date-changed=${ele._beginChange} date=${_toDate(ele._state.begin)}></elix-date-combo-box>
     </label>
     `;
 } else {
   return html`
     <p>Display only the points that have data before the date.</p>
     <label>
       Number of points
       <input @change=${ele._numChanged} type=number value='${ele._state.num_commits}' min=1 max=5000 list=defaultNumbers>
     </label>
     <datalist id=defaultNumbers>
       <option value=50>
       <option value=100>
       <option value=250>
       <option value=500>
     </datalist>
   `;
 }
};

const template = (ele) => html`
  <dialog-sk>
    <h2>Graph Domain</h2>
    <radiogroup>
      <radio-sk @change=${ele._typeRange} ?checked=${ele._state.request_type === 0} label='Date Range'></radio-sk>
      <radio-sk @change=${ele._typeDense} ?checked=${ele._state.request_type === 1} label='Dense'></radio-sk>
    </radiogroup>
    <div>
      ${_request_type(ele)}
    </div>
    <div>
      <label>
        End:
        <elix-date-combo-box @date-changed=${ele._endChange} date=${_toDate(ele._state.end)}></elix-date-combo-box>
      </label>
    </div>
    <div id=controls>
      <button @click=${ele._cancel}>Cancel</button>
      <button @click=${ele._ok} ?disabled=${ele._isInvalid(ele)}>OK</button>
    </div>
  </dialog-sk>
  <button id=description @click=${ele._edit}>${_description(ele)}</button>
`;

window.customElements.define('domain-picker-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {};
    this._description = '';
  }

  connectedCallback() {
    this._render();
    this._dialog = $('dialog-sk', this);
  }

  _typeRange() {
    if (this.force_request_type === 'dense') {
      this._typeDense();
      return
    }
    this._state.request_type = 0;
    this._render();
  }

  _typeDense() {
    if (this.force_request_type === 'range') {
      this._typeRange();
      return
    }
    this._state.request_type = 1;
    this._render();
  }

  _ok() {
    this._dialog.shown = false;
    this.('domain-changed', {state: this._state, bubbles: true});
    const detail = {
      state: this._state,
    }
    this.dispatchEvent(new CustomEvent('domain-changed', {detail: detail, bubbles: true}));
  }

  _beginChange(e) {
    this._state.begin = e.detail.date/1000;
    this._render();
  }

  _endChange(e) {
    this._state.end = e.detail.date/1000;
    this._render();
  }

  _numChanged(e) {
    this._state.num_commits = +e.srcElement.value;
    this._render();
  }

  _edit() {
    this._dialog.shown = true;
  }

  _cancel() {
    this._dialog.shown = false;
  }

  _isInvalid() {
    if (this._state.request_type === 0 && (this._state.end < this._state.begin)) {
      return true;
    }
    return false;
  }

  static get observedAttributes() {
    return ['force_request_type'];
  }

  /** @prop state {Object} An object that contains the following state:
   *
   *  {
   *    begin:         // unix timestamp in seconds.
   *    end:           // unix timestamp in seconds.
   *    num_commits:   // Number of commits.
   *    request_type:  // 0 for date range, 1 for dense. See dataframe.RequestType.
   *  }
   */
  get state() { return this._state }
  set state(val) { this._state = val; }

  /** @prop force_request_type {string} A value of 'dense' or 'range' will force the corresponding request_type to be always set.
  */
  get force_request_type() { return this.getAttribute('force_request_type'); }
  set force_request_type(val) { this.setAttribute('force_request_type', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
