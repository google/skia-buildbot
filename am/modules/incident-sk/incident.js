/**
 * @module incident-sk
 * @description <h2><code>incident-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html/lib/lit-extended'

function table(o) {
  let keys = Object.keys(o);
  keys.sort();
  return keys.map((k) => html`<tr><th>${k}</th><td>${o[k]}</td></tr>`);
}

const template = (ele) => html`
<h2>${ele._state.params.alertname}</h2>
  <table>
  ${table(ele._state.params)}
  </table>
`;

window.customElements.define('incident-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
  }

  /** @prop state {string} An Incident. */
  get state() { return this._state }
  set state(val) {
    this._state = val;
    this._render();
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

});
