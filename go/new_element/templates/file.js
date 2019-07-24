/**
 * @module module/{{.ElementName}}
 * @description <h2><code>{{.ElementName}}</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

const template = (ele) => html``;

window.customElements.define('{{.ElementName}}', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }

});
