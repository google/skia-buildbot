/**
 * @module /named-fiddle-sk
 * @description <h2><code>named-fiddle-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html/lib/lit-extended'
import 'elements-sk/buttons'

const status = (ele) => ele._state.status != 'OK'  ? ele._state.status : '';

const template = (ele) => html`<span class=name><a href='https://fiddle.skia.org/c/${ele._state.hash}'>${ele._state.name}</a></span> <span class=status>${status(ele)}</span> <button data-name$=${ele._state.name}>Edit</button> <button data-name$=${ele._state.name}>Delete</button>`;

window.customElements.define('named-fiddle-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      name: '',
      status: '',
      hash: '',
    };
  }

  /** @prop state {object} A serialized NamedFiddle.  */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

});
