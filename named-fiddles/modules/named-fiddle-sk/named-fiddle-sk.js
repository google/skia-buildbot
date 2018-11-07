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
import { html, render } from 'lit-html'
import 'elements-sk/styles/buttons'

function status(ele) {
  return ele._state.status !== 'OK'  ? ele._state.status : '';
}

const template = (ele) => html`<span class=name><a href='https://fiddle.skia.org/c/${ele._state.hash}'>${ele._state.name}</a></span>
  <span class=status>${status(ele)}</span>
  <button @click=${ele._editClick}>Edit</button>
  <button @click=${ele._deleteClick}>Delete</button>
  <span class=user>${ele._state.user}</span>
`;

window.customElements.define('named-fiddle-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      name: '',
      status: '',
      hash: '',
      user: '',
    };
  }

  _editClick() {
    let detail = Object.assign({}, this._state);
    this.dispatchEvent(new CustomEvent('named-edit', { detail: detail, bubbles: true }));
  }

  _deleteClick() {
    let detail = Object.assign({}, this._state);
    this.dispatchEvent(new CustomEvent('named-delete', { detail: detail, bubbles: true }));
  }


  /** @prop state {object} A serialized Named struct.  */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
