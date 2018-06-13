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
import { repeat } from 'lit-html/lib/repeat';
import '../named-fiddle-sk'
import 'elements-sk/buttons'
import 'common-sk/modules/error-toast-sk'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { errorMessage } from 'common-sk/modules/errorMessage'

const template = (ele) => html`
<header>
  <h1>Named Fiddles</h1>
</header>
<main>
  <section>
   ${repeat(ele._named_fiddles, (i) => i.name, (i, index) => html`<named-fiddle-sk state=${i}></named-fiddle-sk>`)}
  </section>
  <button class=fab>+</button>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
<footer>
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
