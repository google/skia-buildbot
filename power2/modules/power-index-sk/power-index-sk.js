import { html, render } from 'lit-html/lib/lit-extended'

const template = (ele) => html`
<h2>Hello custom element</h2>`;

window.customElements.define('power-index-sk', class extends HTMLElement {

  connectedCallback() {
    this._render();
  }

  _render() {
    render(template(this), this);
  }

});
