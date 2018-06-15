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
import '../named-edit-sk'
import '../named-fiddle-sk'
import 'common-sk/modules/error-toast-sk'
import 'elements-sk/buttons'
import 'elements-sk/spinner-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { repeat } from 'lit-html/lib/repeat';

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
  <named-edit-sk id=editor on-named-edit-complete=${(e) => ele._doUpdate(e.detail)}></named-edit-sk>
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
    this._render();
    this._editor = $$('#editor', this);
    this._busy = $$('#busy', this);
    this._busy.active = true;
    this.addEventListener('named-delete', (e) => this._doDelete(e.detail));
    this.addEventListener('named-edit', (e) => this._edit(e));
    fetch('/_/named').then(jsonOrThrow).then((json) => {
      this._named_fiddles = json;
      this._render();
      this._busy.active = false;
    }).catch((msg) => {
      errorMessage(msg);
      this._busy.active = false;
    });
  }

  _edit(e) {
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

  _doUpdate(detail) {
    this._doImpl("/_/update", detail)
  }

  _doDelete(detail) {
    this._doImpl("/_/delete", detail)
  }

  _doImpl(url, detail) {
    this._busy.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
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
