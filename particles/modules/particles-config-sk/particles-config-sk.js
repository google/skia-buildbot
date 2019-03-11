/**
 * @module particles-config-sk
 * @description <h2><code>particles-config-sk</code></h2>
 *
 * <p>
 *   A dialog for configuring how to render a Particles JSON file.
 * </p>
 *
 * <p>
 *   The form of the 'state' property looks like a serialized UploadRequest:
 * </p>
 * <pre>
 *   {
 *     filename: 'foo.json',
 *     json: {},
 *   }
 * <pre>
 *
 * @evt particles-json-selected - This event is generated when the user presses Go.
 *         The updated state, width, and height is available in the event detail.
 *         There is also an indication if the particles file was changed.
 *
 * @evt cancelled - This event is generated when the user presses Cancel.
 *
 */
import 'elements-sk/styles/buttons'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'

const DEFAULT_SIZE = 600;

const cancelButton = (ele) => ele._hasCancel() ? html`<button id=cancel @click=${ele._cancel}>Cancel</button>` : '';

const template = (ele) => html`
  <label class=file>Particles file to upload
    <input type=file name=file id=file @change=${ele._onFileChange}/>
  </label>
  <div class="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <label class=number>
    <input type=number id=width .value=${ele._width} required /> Width (px)
  </label>
  <label class=number>
    <input type=number id=height .value=${ele._height} required /> Height (px)
  </label>
  <div id=dialog-buttons>
    ${cancelButton(ele)}
    <button class=action ?disabled=${ele._readyToGo()} @click=${ele._go}>Go</button>
  </div>
`;

class ParticlesConfigSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      json: null
    };
    this._width = DEFAULT_SIZE;
    this._height = DEFAULT_SIZE;
    this._fileChanged = false;
    this._starting_state = Object.assign({}, this._state);
  }

  connectedCallback() {
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
  }

  /** @prop height {Number} Selected height for animation. */
  get height() { return this._height; }
  set height(val) {
    this._height= +val;
    this._render();
  }

  /** @prop state {string} Object that describes the state of the config dialog. */
  get state() { return this._state; }
  set state(val) {
    this._state = Object.assign({}, val);
    this._starting_state = Object.assign({}, this._state);
    this._render();
  }

  /** @prop width {Number} Selected width for animation. */
  get width() { return this._width; }
  set width(val) {
    this._width = +val;
    this._render();
  }

  _hasCancel() {
     return !!this._starting_state.json;
  }

  _readyToGo() {
    return !this._state.json;
  }

  _onFileChange(e) {
    this._fileChanged = true;
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
      this._state.json = parsed;
      this._state.filename = e.target.files[0].name;
      this._width = parsed.w || DEFAULT_SIZE;
      this._height = parsed.h || DEFAULT_SIZE;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _updateState() {
    this._width = +$$('#width', this).value;
    this._height = +$$('#height', this).value;
  }

  _go() {
    this._updateState();
    this.dispatchEvent(new CustomEvent('particles-json-selected', { detail: {
      'state' : this._state,
      'fileChanged': this._fileChanged,
      'width' : this._width,
      'height': this._height,
    }, bubbles: true }));
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

window.customElements.define('particles-config-sk', ParticlesConfigSk);