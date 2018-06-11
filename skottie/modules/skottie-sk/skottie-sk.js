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

const displayDialog = (ele) => html`
<skottie-config-sk state=${ele._state}></skottie-config-sk>
`;

const displayLoaded= (ele) => html`
<button class=edit-config on-click=${(e) => ele._startEdit()}>${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...</button>
<video title=lottie autoplay loop src="/_/i/${ele._hash}" width=256 height=256>
  <spinner-sk active></spinner-sk>
</video>
`;

const displayLoading = (ele) => html`
  <div class=loading>
    <spinner-sk active></spinner-sk><span>Loading...</span>
  </div>
`;

const pick = (ele) => {
  switch (ele._ui) {
    case 'dialog':
      return displayDialog(ele);
    case 'loading':
      return displayLoading(ele);
    case 'loaded':
      return displayLoaded(ele);
  }
};

const template = (ele) => html`
<header>
  <h2>Skottie</h2>
</header>
<main>
  ${pick(ele)}
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
      lottie: null,
      width: 256,
      height: 256,
      fps: 30,
    };
    // One of "dialog", "loading", or "loaded"
    this._ui = "dialog";
    this._dialog = true;
    this._loading = false;
    this._hash = '';
  }

  connectedCallback() {
    // Check URL.
    let match = window.location.pathname.match(/\/([a-zA-Z0-9]+)/);
    if (match) {
      // If hash then load from server.
      this._hash = match[1];
      this._ui = 'loading';
      this._render();
      fetch(`/_/j/${this._hash}`).then(jsonOrThrow).then(json => {
        this._state = json;
        this._ui = 'loaded';
        this._render();
      }).catch((msg) => {
        errorMessage(msg);
        window.history.pushState(null, '', '/');
        this._ui = 'dialog';
        this._render();
      });
    }
    this.addEventListener('skottie-selected', this)
    this.addEventListener('cancelled', this)
    this._render();
  }

  disconnectedCallback() {
    this.removeEventListener('skottie-selected', this)
    this.removeEventListener('cancelled', this)
  }

  _startEdit() {
    this._ui = 'dialog';
    this._render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._ui = 'loading';
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
      this._ui = 'loaded';
      this._hash = json.hash;
      window.history.pushState(null, '', '/' + this._hash);
      this._render();
    }).catch(msg => {
      this._ui = 'dialog';
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
      this._ui = 'loaded';
      this._render();
    }
  }

});
