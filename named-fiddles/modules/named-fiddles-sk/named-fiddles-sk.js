/**
 * @module named-fiddles-sk
 * @description <h2><code>named-fiddles-sk</code></h2>
 *
 *   The main application element for named-fiddles.skia.org.
 *
 * @attr csrf - The csrf string to attach to POST requests, based64 encoded.
 */
import '../named-edit-sk';
import '../named-fiddle-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/spinner-sk';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, render } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { repeat } from 'lit-html/directives/repeat';

const template = (ele) => html`
<header>
  <h1>Named Fiddles</h1>
</header>
<main>
  <section>
   ${repeat(ele._named_fiddles, (i) => i.name, (i) => html`<named-fiddle-sk .inflight=${!!ele._inflight[i.name]} .state=${i}></named-fiddle-sk>`)}
  </section>
  <button @click=${ele._new} class=fab>+</button>
  <spinner-sk id=busy></spinner-sk>
  <named-edit-sk id=editor @named-edit-complete=${ele._doUpdate}></named-edit-sk>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
<footer>
`;

define('named-fiddles-sk', class extends HTMLElement {
  constructor() {
    super();
    this._named_fiddles = [];

    // All the named fiddles that are inflight, i.e. we have updated but
    // haven't checked if they've become valid yet.
    this._inflight = {};
  }

  connectedCallback() {
    this._render();
    this._editor = $$('#editor', this);
    this._busy = $$('#busy', this);
    this._busy.active = true;
    this.addEventListener('named-delete', (e) => this._doDelete(e));
    this.addEventListener('named-edit', (e) => this._edit(e));
    this._reloadAll();
  }

  _reloadAll() {
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

  _new() {
    this._editor.state = {
      name: '',
      hash: '',
    };
    this._editor.show();
  }

  _doUpdate(e) {
    this._doImpl('/_/update', e.detail);
    // If the fiddle is failed, then wait a minute and reload
    // the data, since validation should be done by then.
    if (e.detail.status) {
      this._inflight[e.detail.name] = true;
      window.setTimeout(() => {
        this._reloadAll();
        this._inflight[e.detail.name] = false;
      }, 60 * 1000);
    }
  }

  _doDelete(e) {
    this._doImpl('/_/delete', e.detail);
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
    }).then(jsonOrThrow).then((json) => {
      // Should return with updated config.
      this._named_fiddles = json;
      this._render();
      this._busy.active = false;
    }).catch((msg) => {
      this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
});
