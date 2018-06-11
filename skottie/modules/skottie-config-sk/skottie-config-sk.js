/**
 * @module /skottie-config-sk
 * @description <h2><code>skottie-config-sk</code></h2>
 *
 * <p>
 *   A dialog for configuring how to render a lottie file.
 * </p>
 *
 * <p>
 *   The form of the 'state' property looks like a serialized UploadRequest:
 * </p>
 * <pre>
 *   {
 *     filename: 'foo.json',
 *     lottie: {},
 *     width: 256,
 *     height: 256,
 *     fps: 30,
 *   }
 * <pre>
 *
 * @evt skottie-selected - This event is generated when the user presses Go.
 *         The updated state is available in the event detail.
 *
 * @evt cancelled - This event is generated when the user presses Cancel.
 *
 */
import 'elements-sk/buttons'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { $$ } from 'common-sk/modules/dom'

const template = (ele) => html`
  <label class=number>
    <input type=number id=width value=${ele._state.width} required /> Width (px)
    <span class="validity"></span>
  </label>
  <label class=number>
    <input type=number id=height value=${ele._state.height} required /> Height (px)
    <span class="validity"></span>
  </label>
  <label class=number title='Frames Per Second'>
    <input type=number id=fps value=${ele._state.fps} required /> FPS (Hz)
    <span class='validity'></span>
  </label>
  <label class='file'>Lottie file to upload
    <input type=file name=file id=file on-change=${(e) => ele._onFileChange(e)}/>
  </label>
  <div class$="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <div id=dialog-buttons>
    ${ele._hasCancel() ? html`<button id=cancel on-click=${(e) => ele._cancel()}>Cancel</button>` : html`` }
    <button class=action disabled?=${ele._readyToGo()} on-click=${(e) => ele._go()}>Go</button>
  </div>
`;

class SkottieConfigSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: null,
      width: 256,
      height: 256,
      fps: 30,
    };
    this._starting_state = Object.assign({}, this._state);
  }

  connectedCallback() {
    this._render();
  }

  /** @prop state {string} Object that describes the state of the config dialog. */
  get state() { return this._state; }
  set state(val) {
    this._state = Object.assign({}, val);
    this._starting_state = Object.assign({}, this._state);
    this._render();
  }

  _hasCancel() {
     return this._starting_state.lottie != null;
  }

  _readyToGo() {
    return !this._state.lottie;
  }

  _onFileChange(e) {
    let reader = new FileReader();
    reader.addEventListener('load', () => {
      let parsed = {};
      try {
        parsed = JSON.parse(reader.result);
      }
      catch(error) {
        errorMessage(`Not a valid JSON file: ${error}`);
        return;
      }
      this._state.lottie = parsed;
      this._state.filename = e.target.files[0].name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _go() {
    this._state.width = +$$('#width', this).value;
    this._state.height = +$$('#height', this).value;
    this._state.fps = +$$('#fps', this).value;
    this.dispatchEvent(new CustomEvent('skottie-selected', { detail: this._state, bubbles: true }));
  }

  _cancel() {
    this.dispatchEvent(new CustomEvent('cancelled', { bubbles: true }));
  }

  _render() {
    render(template(this), this);
  }
};

window.customElements.define('skottie-config-sk', SkottieConfigSk);
