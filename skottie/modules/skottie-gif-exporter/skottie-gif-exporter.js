/**
 * @module skottie-gif-exporter
 * @description <h2><code>skottie-gif-exporter</code></h2>
 *
 * <p>
 *   A skottie gif exporter
 * </p>
 *
 * @evt start - This event is generated when the saving process starts.
 *
 */
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { html, render } from 'lit-html';
import GIF from './gif';

const QUALITY_SCRUBBER_RANGE = 50;

const WORKERS_COUNT = 4;

const ditherOptions = [
  'FloydSteinberg',
  'FalseFloydSteinberg',
  'Stucki',
  'Atkinson',
];

const exportStates = {
  IDLE: 'idle',
  GIF_PROCESSING: 'gif processing',
  IMAGE_PROCESSING: 'image processing',
  COMPLETE: 'complete',
};

const renderHeader = (ele) => {
  if (ele._state.state === exportStates.IDLE) {
    return html`
      <button @click=${ele._save}>Save</button>
    `;
  }
  return html`
    <button @click=${ele._cancel}>Cancel</button>
  `;
};

const renderOption = (ele, item, index) => html`
  <div
    role="option"
    ?selected=${ele._state.ditherValue === index}
  >
    ${item}
  </div>
`;

const renderRepeatsLabel = (val) => {
  switch (val) {
    case -1:
      return 'No repeats';
    case 0:
      return 'Infinite repeats';
    case 1:
      return `${val} Repeat`;
    default:
      return `${val} Repeats`;
  }
};

const renderDither = (ele) => {
  if (ele._state.dither) {
    return html`
      <select-sk
        role="listbox"
        @selection-changed=${ele._ditherOptionChange}
      >
        ${ditherOptions.map((item, index) => renderOption(ele, item, index))}
      </select-sk>
    `;
  }
  return null;
};

const renderIdle = (ele) => html`
  <div class=form>
    <div class=form-elem>
      <div>Sample (${ele._state.quality})</div>
      <input id=sampleScrub type=range min=1 max=${QUALITY_SCRUBBER_RANGE} step=1
          @input=${ele._onSampleScrub} @change=${ele._onSampleScrubEnd}>
    </div>
    <div class=form-elem>
      <label class=number>
        <input
          type=number
          id=repeats
          .value=${ele._state.repeat}
          min=-1
          @input=${ele._onRepeatChange}
          @change=${ele._onRepeatChange}
        /> Repeats (${renderRepeatsLabel(ele._state.repeat)})
      </label>
    </div>
    <div class=form-elem>
      <checkbox-sk label="Dither"
         ?checked=${ele._state.dither}
         @click=${ele._toggleDither}>
      </checkbox-sk>
      ${renderDither(ele)}
    </div>
  </div>
`;

const renderComplete = (ele) => html`
  <section class=complete>
    <div>
      Render Complete
    </div>
    <a
      class=download
      href=${ele._blobURL}
      download=${ele._getDownloadFileName()}
    >
      Download
    </a>
  </section>
`;

const renderExporting = (text) => html`
  <section class=exporting>
    <div>
      ${text}
    </div>
  </section>
`;

const renderImage = (ele) => renderExporting(`Creating snapshots: ${ele._state.progress}%`);

const renderGif = (ele) => renderExporting(`Creating GIF: ${ele._state.progress}%`);

const mainRenders = {
  [exportStates.IDLE]: renderIdle,
  [exportStates.IMAGE_PROCESSING]: renderImage,
  [exportStates.GIF_PROCESSING]: renderGif,
  [exportStates.COMPLETE]: renderComplete,
};

const renderMain = (ele) => mainRenders[ele._state.state](ele);

const template = (ele) => html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Gif Exporter</div>
      <div class="editor-header-separator"></div>
      ${renderHeader(ele)}
    </header>
    <section class=main>
      ${renderMain(ele)}
    <section>
  </div>
`;

class SkottieGifExporterSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      quality: 50,
      repeat: -1,
      dither: false,
      ditherValue: 0,
      state: exportStates.IDLE,
      progress: 0,
      blob: null,
    };
  }

  delay(time) {
    return new Promise((resolve) => setTimeout(resolve, time));
  }

  _onSampleScrub(ev) {
    this._state.quality = ev.target.value;
    this._render();
  }

  _onSampleScrubEnd(ev) {
    this._state.quality = ev.target.value;
    this._render();
  }

  _onRepeatChange(ev) {
    this._state.repeat = parseInt(ev.target.value, 10);
    this._render();
  }

  _toggleDither(e) {
    e.preventDefault();
    this._state.dither = !this._state.dither;
    this._render();
  }

  _ditherOptionChange(e) {
    e.preventDefault();
    this._state.ditherValue = e.detail.selection;
    this._render();
  }

  _getDownloadFileName() {
    return this.player.animationName() || 'animation.gif';
  }

  /*
  *
  * This method takes care of traversing all frames from the passed animation
  * it adds all frames to the gif instance with a 1 ms delay between frames
  * to prevent blocking the main thread.
  */
  async _processFrames() {
    const fps = this.player.fps();
    const duration = this.player.duration();
    const canvasElement = this.player.canvas();
    let currentTime = 0;
    const increment = 1000 / fps;
    this._state.state = exportStates.IMAGE_PROCESSING;
    this._render();
    while (currentTime < duration) {
      if (this._state.state !== exportStates.IMAGE_PROCESSING) {
        return;
      }
      await this.delay(1); // eslint-disable-line no-await-in-loop
      this.player.seek(currentTime / duration);
      this._gif.addFrame(canvasElement, { delay: increment, copy: true });
      this._state.progress = Math.round((currentTime / duration) * 100);
      currentTime += increment;
      this._render();
    }
    this._state.state = exportStates.GIF_PROCESSING;
    // Note: this render method belongs to the gif.js library, not the html-lit
    this._gif.render();
  }

  _cancel() {
    if (this._state.state === exportStates.GIF_PROCESSING) {
      this._gif.abort();
    }
    this._state.state = exportStates.IDLE;
    this._render();
  }

  _createGifExporter() {
    this._gif = new GIF({
      workers: WORKERS_COUNT,
      quality: this._state.quality,
      repeat: this._state.repeat,
      dither: this._state.dither ? ditherOptions[this._state.ditherValue] : false,
      transparent: 0x00000000,
      workerScript: '/static/gif.worker.js',
    });
    this._gif.on('finished', (blob) => {
      this._state.state = exportStates.COMPLETE;
      this._state.blob = blob;
      this._blobURL = URL.createObjectURL(blob);
      this._render();
    });
    this._gif.on('progress', (value) => {
      this._state.progress = Math.round(value * 100);
      this._render();
    });
  }

  _start() {
    this._state.progress = 0;
    this.dispatchEvent(new CustomEvent('start', {
      detail: '',
    }));
  }

  async _save() {
    this._start();
    this._createGifExporter();
    this._processFrames();
  }

  connectedCallback() {
    this._player = this.player;
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  disconnectedCallback() {
    this.removeEventListener('input', this._inputEvent);
  }

  /** @prop player {skottie-player-sk} Skottie player instance. */
  get player() { return this._player; }

  set player(val) {
    this._player = val;
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
}

define('skottie-gif-exporter', SkottieGifExporterSk);
