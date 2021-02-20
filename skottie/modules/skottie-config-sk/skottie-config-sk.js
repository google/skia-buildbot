/**
 * @module skottie-config-sk
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
 *     assetsZip: 'data:application/zip;base64,...'
 *     assetsFileName: 'assets.zip'
 *   }
 * <pre>
 *
 * @evt skottie-selected - This event is generated when the user presses Go.
 *         The updated state, width, and height is available in the event detail.
 *         There is also an indication if the lottie file was changed.
 *
 * @evt cancelled - This event is generated when the user presses Cancel.
 *
 */
import 'elements-sk/styles/buttons'
import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'

const DEFAULT_SIZE = 128;

const BACKGROUND_VALUES = {
  TRANSPARENT: 'rgba(0,0,0,0)',
  LIGHT: '#FFFFFF',
  DARK: '#000000',
};

const allowZips = window.location.hostname === "skottie-internal.skia.org" ||
                  window.location.hostname === "localhost";

const cancelButton = (ele) => ele._hasCancel() ? html`<button id=cancel @click=${ele._cancel}>Cancel</button>` : '';

const template = (ele) => html`
  <div ?hidden=${!allowZips}>
    We support 3 types of uploads:
    <ul>
      <li>A plain JSON file.</li>
      <li>A JSON file with a zip file of assets (e.g. images) used by the animation.</li>
      <li>
        A zip file produced by lottiefiles.com
        (<a href="https://lottiefiles.com/1187-puppy-run">example</a>)
        with a JSON file in the top level and an images/ directory.
      </li>
    </ul>
  </div>
  <label class=file>Lottie file to upload
    <input type=file name=file id=file @change=${ele._onFileChange}/>
  </label>
  <div class="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <label class=file ?hidden=${!allowZips}>Optional Asset Folder (.zip)
    <input type=file name=folder id=folder @change=${ele._onFolderChange}/>
  </label>
  <div class="filename ${ele._state.assetsFilename ? '' : 'empty'}" ?hidden=${!allowZips}>
    ${ele._state.assetsFilename ? ele._state.assetsFilename : 'No asset folder selected.'}
  </div>
  <label class=number>
    Background Color
    <select id="backgroundColor">
      <option
        value=${BACKGROUND_VALUES.TRANSPARENT}
        ?selected=${ele._backgroundColor === BACKGROUND_VALUES.TRANSPARENT}
      >Transparent</option>
      <option
        value=${BACKGROUND_VALUES.LIGHT}
        ?selected=${ele._backgroundColor === BACKGROUND_VALUES.LIGHT}
      >Light</option>
      <option
        value=${BACKGROUND_VALUES.DARK}
        test=${ele._backgroundColor}
        ?selected=${ele._backgroundColor === BACKGROUND_VALUES.DARK}
       >Dark</option>
    </select>
  </label>
  <label class=number>
    <input type=number id=width .value=${ele._width} required /> Width (px)
  </label>
  <label class=number>
    <input type=number id=height .value=${ele._height} required /> Height (px)
  </label>
  <label class=number>
    <input type=number id=fps .value=${ele._fps} required /> FPS
  </label>
  <div>
    0 for width/height means use the default from the animation. For FPS, 0 means "as smooth as possible"
    and -1 means "use what the animation says".
  </div>
  <div class=warning ?hidden=${ele._warningHidden()}>
    <p>
    The width or height of your file exceeds 1024, which may not fit on the screen.
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
      assetsZip: '',
      assetsFileName: '',
    };
    this._width = DEFAULT_SIZE;
    this._height = DEFAULT_SIZE;
    this._fps = 0;
    this._backgroundColor = BACKGROUND_VALUES.TRANSPARENT;
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

  /** @prop fps {Number} Selected FPS for animation. */
  get fps() { return this._fps; }
  set fps(val) {
    this._fps = +val;
    this._render();
  }

  /** @prop width {Number} Selected width for animation. */
  get width() { return this._width; }
  set width(val) {
    this._width = +val;
    this._render();
  }

  /** @prop backgroundColor {string} Selected background color for animation. */
  get backgroundColor() { return this._backgroundColor; }
  set backgroundColor(val) {
    this._backgroundColor = val;
    this._render();
  }

  _hasCancel() {
     return !!this._starting_state.lottie;
  }

  _readyToGo() {
    return !this._state.filename && (this._state.lottie || this._state.assetsZip);
  }

  _onFileChange(e) {
    this._fileChanged = true;
    const toLoad = e.target.files[0];
    const reader = new FileReader();
    if (toLoad.name.endsWith('.json')) {
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
        this._state.filename = toLoad.name;
        this._width = parsed.w || DEFAULT_SIZE;
        this._height = parsed.h || DEFAULT_SIZE;
        this._render();
      });
      reader.addEventListener('error', () => {
        errorMessage('Failed to load.');
      });
      reader.readAsText(toLoad);
    } else if (allowZips && toLoad.name.endsWith('.zip')) {
      reader.addEventListener('load', () => {
        this._state.lottie = '';
        this._state.assetsZip = reader.result;
        this._state.filename = toLoad.name;

        this._width = DEFAULT_SIZE;
        this._height = DEFAULT_SIZE;
        this._render();
      });
      reader.addEventListener('error', () => {
        errorMessage('Failed to load '+ toLoad.name);
      });
      reader.readAsDataURL(toLoad);
    } else {
      let msg = `Bad file type ${toLoad.name}, only .json and .zip supported`;
      if (!allowZips) {
        msg = `Bad file type ${toLoad.name}, only .json supported`;
      }
      errorMessage(msg);
      this._state.filename = '';
      this._state.lottie = '';
    }
  }

  _onFolderChange(e) {
    this._fileChanged = true;
    const toLoad = e.target.files[0];
    const reader = new FileReader();
    reader.addEventListener('load', () => {
      this._state.assetsZip = reader.result;
      this._state.assetsFilename = toLoad.name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load '+ toLoad.name);
    });
    reader.readAsDataURL(toLoad);
  }

  _rescale(n) {
    let max = Math.max(this._width, this._height);
    if (max <= n) {
      return
    }
    this._width = Math.floor(this._width * n / max);
    this._height = Math.floor(this._height * n / max);
    this._render();
  }

  _warningHidden() {
    return this._width <= 1024 && this._width <= 1024;
  }

  _updateState() {
    this._width = +$$('#width', this).value;
    this._height = +$$('#height', this).value;
    this._fps = +$$('#fps', this).value;
    this._backgroundColor = $$('#backgroundColor', this).value;
  }

  _go() {
    this._updateState();
    this.dispatchEvent(new CustomEvent('skottie-selected', { detail: {
      'state' : this._state,
      'fileChanged': this._fileChanged,
      'width' : this._width,
      'height': this._height,
      'fps': this._fps,
      'backgroundColor': this._backgroundColor,
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

define('skottie-config-sk', SkottieConfigSk);
