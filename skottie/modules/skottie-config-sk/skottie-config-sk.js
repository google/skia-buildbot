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
import 'elements-sk/styles/buttons'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'

const DEFAULT_SIZE = 128;
const DEFAULT_FPS = 29.97;

const cancelButton = (ele) => ele._hasCancel() ? html`<button id=cancel @click=${ele._cancel}>Cancel</button>` : '';

const template = (ele) => html`
  <label class=file>Lottie file to upload
    <input type=file name=file id=file @change=${ele._onFileChange}/>
  </label>
  <div class="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <label class=number>
    <input type=number id=width value=${ele._state.width} required /> Width (px)
  </label>
  <label class=number>
    <input type=number id=height value=${ele._state.height} required /> Height (px)
  </label>
  <label class=number title='Frames Per Second'>
    <input type=number id=fps value=${ele._state.fps} required  step='0.01'/> FPS (Hz)
  </label>
  <div class=warning ?hidden=${ele._warningHidden()}>
    <p>
    The width or height of your file exceeds 1024, which will be very slow to render.
    Press a 'Rescale' button to fix the dimensions while preserving the aspect ratio.
    </p>
    <div>
      <button @click=${(e) => ele._rescale(1024)}>Rescale to 1024</button>
      <button @click=${(e) => ele._rescale(512)}>Rescale to 512</button>
      <button @click=${(e) => ele._rescale(128)}>Rescale to 128</button>
    </div>
  </div>
  <div id=dialog-buttons>
    ${cancelButton(ele)}
    <button class=action ?disabled=${ele._readyToGo()} @click=${ele._go}>Go</button>
  </div>
`;

class SkottieConfigSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: null,
      width: DEFAULT_SIZE,
      height: DEFAULT_SIZE,
      fps: DEFAULT_FPS,
    };
    this._starting_state = Object.assign({}, this._state);
  }

  connectedCallback() {
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
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
      this._state.width = parsed.w || DEFAULT_SIZE;
      this._state.height = parsed.h || DEFAULT_SIZE;
      this._state.fps = parsed.fr || DEFAULT_FPS;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _rescale(n) {
    let max = Math.max(this._state.width, this._state.height);
    if (max <= n) {
      return
    }
    this._state.width = Math.floor(this._state.width * n / max);
    this._state.height = Math.floor(this._state.height * n / max);
    this._render();
  }

  _warningHidden() {
    return this._state.width <= 1024 && this._state.width <= 1024;
  }

  _updateState() {
    this._state.width = +$$('#width', this).value;
    this._state.height = +$$('#height', this).value;
    this._state.fps = +$$('#fps', this).value;
  }

  _go() {
    this._updateState();
    this.dispatchEvent(new CustomEvent('skottie-selected', { detail: this._state, bubbles: true }));
  }

  _cancel() {
    this.dispatchEvent(new CustomEvent('cancelled', { bubbles: true }));
  }

  _inputEvent() {
    this._updateState();
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }
};

window.customElements.define('skottie-config-sk', SkottieConfigSk);
