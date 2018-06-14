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
import '../named-edit-sk'
import { $$ } from 'common-sk/modules/dom'
import 'elements-sk/spinner-sk'

const template = (ele) => html`
<header>
  <h1>Named Fiddles</h1>
</header>
<main>
  <section>
   ${repeat(ele._named_fiddles, (i) => i.name, (i, index) => html`<named-fiddle-sk state=${i}></named-fiddle-sk>`)}
  </section>
  <div class=buttons>
    <span></span>
    <spinner-sk id=busy></spinner-sk>
    <button on-click=${(e) => ele._new(e)} class=fab>+</button>
  </div>
  <named-edit-sk id=editor on-named-edit-complete=${(e) => ele._doUpdate(e)}></named-edit-sk>
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
    this.addEventListener('named-delete', (e) => this._delete(e));
    this.addEventListener('named-edit', (e) => this._edit(e));
    this._editor = $$('#editor', this);
    this._busy = $$('#busy', this);
  }

  _delete(e) {
    console.log(e.detail);
  }

  _edit(e) {
    console.log(e.detail);
    this._editor.state = e.detail;
    this._editor.show();
  }

  _new(e) {
    this._editor.state = {
      name: '',
      hash: '',
    };
    this._editor.show();
  }

  _doUpdate(e) {
    this._busy.active = true;
    fetch("/_/update", {
      body: JSON.stringify(e.details),
      headers: {
        'content-type': 'application/json'
      },
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with updated config.
      this._named_fiddles = json;
      this._render();
      this._busy.active = false;
    }).catch(msg => {
      errorMessage(msg);
      this._busy.active = false;
    });
  }

  _render() {
    render(template(this), this);
  }

});
