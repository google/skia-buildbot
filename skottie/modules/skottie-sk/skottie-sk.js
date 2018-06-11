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
import '../skottie-config-sk'
import 'common-sk/modules/error-toast-sk'
import 'elements-sk/spinner-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

const displayTemplate = (ele) => html`
<button class=edit-config on-click=${(e) => ele._startEdit()}>${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...</button>
<video title=lottie autoplay loop src="/_/i/${ele._hash}" width=256 height=256>
  <spinner-sk active></spinner-sk>
</video>
`;

const displayDialog = (ele) => html`
<skottie-config-sk state=${ele._state}></skottie-config-sk>
`;

const template = (ele) => html`
<header>
  <h2>Skottie</h2>
</header>
<main>
  ${ele._dialog ?  displayDialog(ele): displayTemplate(ele)}
  <spinner-sk active?=${ele._loading}></spinner-sk>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

window.customElements.define('skottie-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: '',
      width: 256,
      height: 256,
      fps: 30,
    };
    this._dialog = true;
    this._loading = false;
    this._hash = '';
  }

  connectedCallback() {
    // Check URL.
    // If hash then load from server.
    this._config = $$('skottie-config-sk', this);
    this.addEventListener('skottie-selected', this)
    this.addEventListener('cancelled', this)
    this._render();
  }

  disconnectedCallback() {
    this.removeEventListener('skottie-selected', this)
    this.removeEventListener('cancelled', this)
  }

  _startEdit() {
    this._dialog = true;
    this._render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._loading = true;
    this._hash = '';
    this._render();
    fetch("/_/upload", {
      body: JSON.stringify(this._state),
      headers: {
        'content-type': 'application/json'
      },
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      // Should return with the hash.
      this._loading = false;
      this._dialog = false;
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

  handleEvent(e) {
    if (e.type == 'skottie-selected') {
      this._state = e.detail;
      this._upload();
    } else if (e.type == 'cancelled') {
      this._dialog = false;
      this._render();
    }
  }

});
