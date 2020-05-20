/**
 * @module named-fiddle-sk
 * @description <h2><code>named-fiddle-sk</code></h2>
 *
 *   Represents a single named fiddle with controls
 *   for manipulating it.
 *
 * @evt named-edit - Sent when the user presses the edit button.
 *   The event detail will be the current state of the named
 *   fiddle.
 *
 * @evt named-delete - Sent when the user presses the delete button.
 *   The event detail will be the current state of the named
 *   fiddle.
 *
 */
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import 'elements-sk/styles/buttons';

function statusValue(ele) {
  return ele._state.status !== '' ? 'Failed' : '';
}

function statusClass(ele) {
  return ele._inflight ? 'inflight' : 'status';
}

const template = (ele) => html`<span class=name><a href='https://fiddle.skia.org/c/${ele._state.hash}'>${ele._state.name}</a></span>
  <span class=${statusClass(ele)} title=${ele._state.status}>${statusValue(ele)}</span>
  <button @click=${ele._editClick}>Edit</button>
  <button @click=${ele._deleteClick}>Delete</button>
  <span class=user>${ele._state.user}</span>
`;

define('named-fiddle-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      name: '',
      status: '',
      hash: '',
      user: '',
    };
    this._inflight = false;
  }

  _editClick() {
    const detail = Object.assign({}, this._state);
    this.dispatchEvent(new CustomEvent('named-edit', { detail: detail, bubbles: true }));
  }

  _deleteClick() {
    const detail = Object.assign({}, this._state);
    this.dispatchEvent(new CustomEvent('named-delete', { detail: detail, bubbles: true }));
  }


  /** @prop state {object} A serialized Named struct.  */
  get state() { return this._state; }

  set state(val) {
    this._state = val;
    this._render();
  }

  /** @prop inflight {Boolean} Is there a request inflight to determine if this fiddle is still invalid.  */
  get inflight() { return this._inflight; }

  set inflight(val) {
    this._inflight = val;
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
});
