/**
 * @module skottie-sk
 * @description <h2><code>skottie-sk</code></h2>
 *
 * <p>
 *   The main application element for skottie.
 * </p>
 *
 */
import '../skottie-config-sk'
import 'common-sk/modules/error-toast-sk'
import 'elements-sk/spinner-sk'
import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

const DIALOG_MODE = 1;
const LOADING_MODE = 2;
const LOADED_MODE = 3;

const displayDialog = (ele) => html`
<skottie-config-sk state=${ele._state}></skottie-config-sk>
`;

const displayLoaded= (ele) => html`
<button class=edit-config on-click=${(e) => ele._startEdit()}>${ele._state.filename} ${ele._state.width}x${ele._state.height} ${ele._state.fps} fps ...</button>
<video title=lottie autoplay loop src="/_/i/${ele._hash}" width=${ele._state.width} height=${ele._state.height}>
  <spinner-sk active></spinner-sk>
</video>
`;

const displayLoading = (ele) => html`
  <div class=loading>
    <spinner-sk active></spinner-sk><span>Loading...</span>
  </div>
`;

// pick the right part of the UI to display based on ele._ui.
const pick = (ele) => {
  switch (ele._ui) {
    case DIALOG_MODE:
      return displayDialog(ele);
    case LOADING_MODE:
      return displayLoading(ele);
    case LOADED_MODE:
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
    this._ui = DIALOG_MODE;
    this._hash = '';
  }

  connectedCallback() {
    this._reflectFromURL();
    this.addEventListener('skottie-selected', this)
    this.addEventListener('cancelled', this)
    window.addEventListener('popstate', this)
    this._render();
  }

  disconnectedCallback() {
    this.removeEventListener('skottie-selected', this)
    this.removeEventListener('cancelled', this)
  }

  _reflectFromURL() {
    // Check URL.
    let match = window.location.pathname.match(/\/([a-zA-Z0-9]+)/);
    if (match) {
      // If hash then load from server.
      this._hash = match[1];
      this._ui = LOADING_MODE;
      this._render();
      fetch(`/_/j/${this._hash}`).then(jsonOrThrow).then(json => {
        this._state = json;
        this._ui = LOADED_MODE;
        this._render();
      }).catch((msg) => {
        errorMessage(msg);
        window.history.pushState(null, '', '/');
        this._ui = DIALOG_MODE;
        this._render();
      });
    } else {
      this._startEdit();
    }
  }

  _startEdit() {
    this._ui = DIALOG_MODE;
    this._render();
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._ui = LOADING_MODE;
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
      this._ui = LOADED_MODE;
      this._hash = json.hash;
      window.history.pushState(null, '', '/' + this._hash);
      this._render();
    }).catch(msg => {
      this._ui = DIALOG_MODE;
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
      this._ui = LOADED_MODE;
      this._render();
    } else if (e.type == 'popstate') {
      this._reflectFromURL();
    }
  }

});
