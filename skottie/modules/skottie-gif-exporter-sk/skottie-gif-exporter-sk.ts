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
import { html } from 'lit-html';
import { bytes, diffDate } from 'common-sk/modules/human';
import { SelectSkSelectionChangedEventDetail } from 'elements-sk/select-sk/select-sk';
import gifStorage from '../helpers/gifStorage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';

interface GIFLibraryOptions {
  workerScript?: string;
  workers?: number;
  repeat?: number;
  background?: string;
  quality?: number;
  width?: number;
  height?: number;
  transparent?: number | null;
  debug?: boolean;
  dither?: boolean | string;
}

interface GIFLibrary {
  abort(): unknown;
  addFrame(canvasElement: HTMLCanvasElement|ImageData|CanvasRenderingContext2D, options: { delay: number; copy: boolean; }): void;
  on(event: string, callback: (_: unknown)=> void): void;
  render(): unknown;
}

interface BackgroundOption {
  id: string;
  label: string;
  color: string;
}

// eslint-disable-next-line @typescript-eslint/no-var-requires
const GIF: new (_: GIFLibraryOptions)=> GIFLibrary = require('./gif.js');

const QUALITY_SCRUBBER_RANGE = 50;

const WORKERS_COUNT = 4;

const ditherOptions = [
  'FloydSteinberg',
  'FalseFloydSteinberg',
  'Stucki',
  'Atkinson',
];

const backgroundOptions: BackgroundOption[] = [
  {
    id: '1',
    label: 'White',
    color: '#ffffff',
  },
  {
    id: '2',
    label: 'Black',
    color: '#000000',
  },
];

type ExportState = 'idle' | 'gif processing' | 'image processing' | 'complete';

const renderRepeatsLabel = (val: number) => {
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

class SkottieGifExporterSk extends ElementSk {
  private static template = (ele: SkottieGifExporterSk) => html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Gif Exporter</div>
      <div class="editor-header-separator"></div>
      ${ele.renderHeader()}
    </header>
    <section class=main>
      ${ele.renderMain()}
    </section>
  </div>
`;

  private renderMain = () => {
    switch (this.state) {
      default:
      case 'idle': return this.renderIdle();
      case 'image processing': return this.renderImage();
      case 'gif processing': return this.renderGif();
      case 'complete': return this.renderComplete();
    }
  }

private renderIdle = () => html`
  <div class=form>
    <div class=form-elem>
      <div>Sample (${this.quality})</div>
      <input id=sampleScrub type=range min=1 max=${QUALITY_SCRUBBER_RANGE} step=1
          @input=${this.updateQuality} @change=${this.updateQuality}
          .value=${this.quality}>
    </div>
    <div class=form-elem>
      <label class=number>
        <input
          type=number
          id=repeats
          .value=${this.repeat}
          min=-1
          @input=${this.onRepeatChange}
          @change=${this.onRepeatChange}
        /> Repeats (${renderRepeatsLabel(this.repeat)})
      </label>
    </div>
    <div class=form-elem>
      <checkbox-sk label="Dither"
         ?checked=${this.dither}
         @click=${this.toggleDither}>
      </checkbox-sk>
      ${this.renderDither()}
    </div>
    <div class=form-elem>
      <checkbox-sk label="Include Transparent Background"
         ?checked=${this.transparent}
         @click=${this.toggleTransparent}>
      </checkbox-sk>
    </div>
    <div class=form-elem>
      <div class=form-elem-label>Select Background Color to compose on Transparent</div>
      ${this.renderBackgroundSelect()}
    </div>
  </div>
`;

  private renderImage = () => html`
  <section class=exporting>
    <div>
      Creating snapshots: ${this.progress}%
    </div>
  </section>
`;

  private renderGif = () => html`
  <section class=exporting>
    <div>
      Creating GIF: ${this.progress}%
    </div>
  </section>
`;

  renderComplete = () => html`
  <section class=complete>
    <div class=export-info>
      <div class=export-info-row>
        Render Complete
      </div>
      <div class=export-info-row>
        Export Duration: ${this.exportDuration}
      </div>
      <div class=export-info-row>
        File size: ${bytes(this.blob ? this.blob.size : 0)}
      </div>
    </div>
    <a
      class=download
      href=${this.blobURL}
      download=${this.getDownloadFileName()}
    >
      Download
    </a>
  </section>
`;

  private renderHeader = () => {
    if (this.state === 'idle') {
      return html`
      <button class="editor-header-save-button" @click=${this.save}>Save</button>
    `;
    }
    if (this.state === 'complete') {
      return html`
      <button class="editor-header-save-button" @click=${this.cancel}>Back</button>
    `;
    }
    return html`
    <button class="editor-header-save-button" @click=${this.cancel}>Cancel</button>
  `;
  };

  private renderDither = () => {
    if (this.dither) {
      return html`
      <select-sk
        role="listbox"
        @selection-changed=${this.ditherOptionChange}
      >
        ${ditherOptions.map((item: string, index: number) => this.renderOption(item, index))}
      </select-sk>
    `;
    }
    return null;
  };

  private renderOption = (item: string, index: number) => html`
  <div
    role="option"
    ?selected=${this.ditherValue === index}
  >
    ${item}
  </div>
`;

  private renderBackgroundOption = (item: BackgroundOption) => html`
  <div
    role="option"
    ?selected=${this.backgroundValue.id === item.id}
  >
    ${item.label}
  </div>
`;

  private renderBackgroundSelect = () => html`
  <select-sk
    role="listbox"
    @selection-changed=${this.backgroundOptionChange}
  >
    ${backgroundOptions.map((item: BackgroundOption) => this.renderBackgroundOption(item))}
  </select-sk>
`;

  private backgroundValue: BackgroundOption = backgroundOptions[0];

  private backgroundValueIndex: number = 0;

  private blob: Blob | null = null;

  private dither: boolean | string = false;

  private ditherValue: number = 0;

  private exportDuration: string = '';

  private gif: GIFLibrary | null = null;

  private progress: number = 0;

  private quality: number = 0;

  private repeat: number = 0;

  private state: ExportState = 'idle';

  private transparent: boolean = false;

  private startTime: number = 0;

  private blobURL: string = '';

  private _player: SkottiePlayerSk | null = null;

  constructor() {
    super(SkottieGifExporterSk.template);
    this.repeat = gifStorage.get('repeat', 0);
    this.quality = gifStorage.get('quality', 50);
    this.backgroundValueIndex = gifStorage.get('backgroundIndex', 0);
    this.transparent = gifStorage.get('transparent', true);
    this.dither = gifStorage.get('dither', false);
    this.ditherValue = gifStorage.get('ditherValue', 0);
    this.backgroundValue = backgroundOptions[this.backgroundValueIndex];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private delay(time: number) {
    return new Promise((resolve) => setTimeout(resolve, time));
  }

  private updateQuality(ev: Event): void {
    this.quality = +(ev.target as HTMLInputElement).value;
    gifStorage.set('quality', this.quality);
    this._render();
  }

  private onRepeatChange(ev: Event) {
    this.repeat = +(ev.target as HTMLInputElement).value;
    gifStorage.set('repeat', this.repeat);
    this._render();
  }

  private toggleDither(e: Event) {
    e.preventDefault();
    this.dither = !this.dither;
    gifStorage.set('dither', this.dither);
    this._render();
  }

  private toggleTransparent(e: Event) {
    e.preventDefault();
    this.transparent = !this.transparent;
    gifStorage.set('transparent', this.transparent);
    this._render();
  }

  private ditherOptionChange(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    e.preventDefault();
    this.ditherValue = e.detail.selection;
    gifStorage.set('ditherValue', this.ditherValue);
    this._render();
  }

  private backgroundOptionChange(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    e.preventDefault();
    this.backgroundValue = backgroundOptions[e.detail.selection];
    gifStorage.set('backgroundIndex', e.detail.selection);
    this._render();
  }

  private getDownloadFileName() {
    return this.player?.animationName() || 'animation.gif';
  }

  /*
  *
  * This method takes care of traversing all frames from the passed animation
  * it adds all frames to the gif instance with a 1 ms delay between frames
  * to prevent blocking the main thread.
  */
  private async processFrames() {
    const fps = this.player.fps();
    const duration = this.player.duration();
    const canvasElement = this.player.canvas();
    let currentTime = 0;
    const increment = 1000 / fps;
    this.state = 'image processing';
    this._render();
    while (currentTime < duration) {
      if (this.state !== 'image processing') {
        return;
      }
      await this.delay(1); // eslint-disable-line no-await-in-loop
      this.player.seek(currentTime / duration, true);
      this.gif!.addFrame(canvasElement!, { delay: increment, copy: true });
      this.progress = Math.round((currentTime / duration) * 100);
      currentTime += increment;
      this._render();
    }
    this.state = 'gif processing';
    // Note: this render method belongs to the gif.js library, not the html-lit
    this.gif!.render();
  }

  private cancel() {
    if (this.state === 'gif processing') {
      this.gif!.abort();
    }
    this.state = 'idle';
    this._render();
  }

  private createGifExporter() {
    this.gif = new GIF({
      workers: WORKERS_COUNT,
      quality: this.quality,
      repeat: this.repeat,
      dither: this.dither ? ditherOptions[this.ditherValue] : false,
      transparent: this.transparent ? 0x00000000 : undefined,
      background: this.backgroundValue.color,
      workerScript: '/static/gif.worker.js',
    });
    this.gif.on('finished', (blob: unknown) => {
      this.state = 'complete';
      this.blob = blob as Blob;
      this.exportDuration = diffDate(this.startTime);
      this.blobURL = URL.createObjectURL(blob);
      this._render();
    });
    this.gif.on('progress', (value: unknown) => {
      this.progress = Math.round((value as number) * 100);
      this._render();
    });
  }

  private start() {
    this.progress = 0;
    this.startTime = Date.now();
    this.dispatchEvent(new CustomEvent('start'));
  }

  private async save(): Promise<void> {
    this.start();
    this.createGifExporter();
    await this.processFrames();
  }

  get player(): SkottiePlayerSk { return this._player!; }

  set player(val: SkottiePlayerSk) {
    this._player = val;
  }
}

define('skottie-gif-exporter-sk', SkottieGifExporterSk);
