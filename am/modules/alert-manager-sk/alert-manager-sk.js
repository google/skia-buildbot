/**
 * @module alert-manager-sk
 * @description <h2><code>alert-manager-sk</code></h2>
 *
 *   The main application element for alert-manager.skia.org.
 *
 * @attr csrf - The csrf string to attach to POST requests, based64 encoded.
 */
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
  <h1>Alerts</h1>
</header>
<main>
  <section>
   ${repeat(ele._incidents, (i) => i.name, (i, index) => html`<named-fiddle-sk state=${i}></named-fiddle-sk>`)}
  </section>
  <spinner-sk id=busy></spinner-sk>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
<footer>
`;

window.customElements.define('alert-manager-sk', class extends HTMLElement {
  constructor() {
    super();
    this._incidents = [];
  }

  connectedCallback() {
    this._render();
    this._busy = $$('#busy', this);
    this._busy.active = true;
    fetch('/_/incidents', {
      credentials: 'include',
    }).then(jsonOrThrow).then((json) => {
      this._incidents = json;
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
      this._incidents = json;
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
