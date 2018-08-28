/**
 * @module named-fiddles-sk
 * @description <h2><code>named-fiddles-sk</code></h2>
 *
 *   The main application element for named-fiddles.skia.org.
 *
 * @attr csrf - The csrf string to attach to POST requests, based64 encoded.
 */
import '../named-edit-sk'
import '../named-fiddle-sk'
import 'elements-sk/error-toast-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/spinner-sk'
import 'infra-sk/modules/login-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { repeat } from 'lit-html/lib/repeat';

const template = (ele) => html`
<header>
  <h1>Named Fiddles</h1>
  <login-sk></login-sk>
</header>
<main>
  <section>
   ${repeat(ele._named_fiddles, (i) => i.name, (i, index) => html`<named-fiddle-sk state=${i}></named-fiddle-sk>`)}
  </section>
  <button on-click=${(e) => ele._new(e)} class=fab>+</button>
  <spinner-sk id=busy></spinner-sk>
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
    fetch('/_/named', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._named_fiddles = json;
      this._render();
      this._busy.active = false;
    }).catch((msg) => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
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
        'content-type': 'application/json',
        'X-CSRF-Token': atob(this.getAttribute('csrf')),
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with updated config.
      this._named_fiddles = json;
      this._render();
      this._busy.active = false;
    }).catch(msg => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  _render() {
    render(template(this), this);
  }

});
