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
import { errorMessage } from 'common-sk/modules/errorMessage'

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
  <button class=action disabled>Go</button>
  <input type=file name=file id=file on-change=${(e) => ele.onFileChange(e)}/>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>
`;

window.customElements.define('skottie-sk', class extends HTMLElement {
  constructor() {
    super();
    this._filename = 'No file currently selected.'
  }

  connectedCallback() {
    this._render();
  }

  disconnectedCallback() {
  }

  onFileChange(e) {
    let reader = new FileReader();
    reader.addEventListener('load', () => {
      try {
        this._lottie = JSON.parse(reader.result);
      }
      catch(error) {
        errorMessage(`Not a valid JSON file: ${error}`);
        return;
      }
      this._filename = e.target.files[0].name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _render() {
    render(template(this), this);
  }

});
