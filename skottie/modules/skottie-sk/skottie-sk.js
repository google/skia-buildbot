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
import { html, render } from 'lit-html/lib/lit-extended'
import 'elements-sk/buttons'
import 'elements-sk/spinner-sk'
import 'common-sk/modules/error-toast-sk'
import { $$ } from 'common-sk/modules/dom'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { errorMessage } from 'common-sk/modules/errorMessage'

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
  <h2>Skottie allows you to view a lottie file played through Skia.</h2>
  <label for=file>Lottie file to upload</label>
  <div class=filename>
    ${ele._filename}
  </div>
  <button class=action disabled?=${!ele._lottie} on-click=${(e) => ele._upload()}>Go</button>
  <input type=file name=file id=file on-change=${(e) => ele._onFileChange(e)}/>
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
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }

  _onFileChange(e) {
    let reader = new FileReader();
    reader.addEventListener('load', () => {
      try {
        JSON.parse(reader.result);
      }
      catch(error) {
        errorMessage(`Not a valid JSON file: ${error}`);
        return;
      }
      this._lottie = reader.result;
      this._filename = e.target.files[0].name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _upload() {
    // POST the JSON along with options to /_/upload
    this._loading = true;
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
