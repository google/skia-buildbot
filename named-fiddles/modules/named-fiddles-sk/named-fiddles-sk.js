/**
 * @module /named-fiddles-sk
 * @description <h2><code>named-fiddles-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html/lib/lit-extended'
import 'elements-sk/buttons';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'common-sk/modules/errorMessage';

const template = (ele) => html`
<main>
  <section>
  ${ele._named_fiddles.map((s) => html`<named-fiddle-sk state=${s}></named-fiddle-sk> `)}
  </section>
  <button class=fab></button>
</main>
`;

window.customElements.define('named-fiddles-sk', class extends HTMLElement {
  constructor() {
    super();
    this._named_fiddles = [];
  }

  connectedCallback() {
    fetch('/_/named').then(jsonOrThrow).then((json) => {
      this._named_fiddles = json;
      this._render();
    }).catch(errorMessage);
    this._render();
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

});
