/**
 * @module skottie-sk
 * @description <h2><code>skottie-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import 'common-sk/modules/error-toast-sk'
import 'elements-sk/spinner-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

const displayTemplate = (ele) => html`
<div>
  <video title=lottie autoplay loop src="/_/i/${ele._hash}" width=256 height=256></video>
</div>
`;

const template = (ele) => html`
<header>
  <h2>Skottie</h2>
</header>
<main>
  <skottie-config-sk></skottie-config-sk>
  <spinner-sk active?=${ele._loading}></spinner-sk>
  ${ele._hash ? displayTemplate(ele) : '' }
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

window.customElements.define('skottie-sk', class extends HTMLElement {
  constructor() {
    super();
    this._filename = 'No file currently selected.'
    this._lottie = null;
    this._loading = false;
    this._hash = '';
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }


  _upload() {
    // POST the JSON along with options to /_/upload
    this._loading = true;
    this._hash = '';
    this._render();
    let data = {
      lottie: this._lottie,
      width: 256,
      height: 256,
      fps: 30,
    };
    fetch("/_/upload", {
      body: JSON.stringify(data),
      headers: {
        'content-type': 'application/json'
      },
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with the hash.
      this._loading = false;
      this._hash = json.hash;
      this._render();
    }).catch(msg => {
      this._loading = false;
      this._render();
      errorMessage(msg);
    });
  }

  _render() {
    render(template(this), this);
  }

});
