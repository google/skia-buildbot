/**
 * @module /skottie-config-sk
 * @description <h2><code>skottie-config-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import 'elements-sk/buttons'
import { errorMessage } from 'common-sk/modules/errorMessage'
import { html, render } from 'lit-html/lib/lit-extended'
import { $ } from 'common-sk/modules/dom'

const cancelButtonTemplate = (ele) => html`
  <button id=cancel on-click=${(e) => ele._cancel()}>Cancel</button>
`;

const template = (ele) => html`
  <label class=number>
    <input type=number id=width value=${ele._state.width} min=1 max=1024 required /> Width (px)
    <span class="validity"></span>
  </label>
  <label class=number>
    <input type=number id=height value=${ele._state.height} min=1 max=1024 required /> Height (px)
    <span class="validity"></span>
  </label>
  <label class=number title='Frames Per Second'>
    <input type=number id=fps value=${ele._state.fps} min=1 max=120 required /> FPS (Hz)
    <span class="validity"></span>
  </label>
  <label for=file>Lottie file to upload</label>
  <div class$="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <div id=dialog-buttons>
    ${ele._starting_state.filename ? html`<button id=cancel on-click=${(e) => ele._cancel()}>Cancel</button>` : html`` }
    <button class=action disabled?=${ele._readyToGo()} on-click=${(e) => ele._go()}>Go</button>
  </div>
  <input type=file name=file id=file on-change=${(e) => ele._onFileChange(e)}/>
`;

class SkottieConfigSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      filename: '',
      lottie: '',
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
    this._state = val;
    this._starting_state = Object.assign({}, this._state);
    this._render();
  }

  _readyToGo() {
    return !this._state.lottie;
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
      this._state.lottie = reader.result;
      this._state.filename = e.target.files[0].name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage('Failed to load.');
    });
    reader.readAsText(e.target.files[0]);
  }

  _go() {
    this.dispatchEvent(new CustomEvent('skottie-selected', { detail: this._state, bubbles: true }));
  }

  _cancel() {
    this.dispatchEvent(new CustomEvent('cancelled', { bubbles: true }));
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

};

window.customElements.define('skottie-config-sk', SkottieConfigSk);
