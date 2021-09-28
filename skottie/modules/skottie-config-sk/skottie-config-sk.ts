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
import 'elements-sk/styles/buttons';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SoundMap } from '../audio';
import { LottieAnimation } from '../types';

const DEFAULT_SIZE = 128;

const BACKGROUND_VALUES = {
  TRANSPARENT: 'rgba(0,0,0,0)',
  LIGHT: '#FFFFFF',
  DARK: '#000000',
};

const allowZips = window.location.hostname === 'skottie-internal.skia.org'
                  || window.location.hostname === 'localhost';

export interface SkottieConfigState {
  assets?: Record<string, ArrayBuffer>;
  filename: string,
  lottie: LottieAnimation | null,
  assetsZip: string,
  assetsFilename: string,
  w?: number,
  h?: number,
  soundMap?: SoundMap,
}

export interface SkottieConfigEventDetail {
  state: SkottieConfigState,
  fileChanged: boolean,
  width: number,
  height: number,
  fps: number,
  backgroundColor: string,
}

export class SkottieConfigSk extends ElementSk {
  private static template = (ele: SkottieConfigSk) => html`
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
    <input type=file name=file id=file @change=${ele.onFileChange}/>
  </label>
  <div class="filename ${ele._state.filename ? '' : 'empty'}">
    ${ele._state.filename ? ele._state.filename : 'No file selected.'}
  </div>
  <label class=file ?hidden=${!allowZips}>Optional Asset Folder (.zip)
    <input type=file name=folder id=folder @change=${ele.onFolderChange}/>
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
        ?selected=${ele._backgroundColor === BACKGROUND_VALUES.DARK}
       >Dark</option>
    </select>
  </label>
  <checkbox-sk label="Lock aspect ratio"
                ?checked=${ele._isRatioLocked}
                @click=${ele.toggleRatioLock}>
    </checkbox-sk>
  <label class=number>
    <input type=number id=width @change=${ele.onWidthInput}
                       .value=${ele._width} required /> Width (px)
  </label>
  <label class=number>
    <input type=number id=height @change=${ele.onHeightInput}
                       .value=${ele._height} required /> Height (px)
  </label>
  <label class=number>
    <input type=number id=fps .value=${ele._fps} required /> FPS
  </label>
  <div>
    0 for width/height means use the default from the animation. For FPS, 0 means "as smooth as possible"
    and -1 means "use what the animation says".
  </div>
  <div class=warning ?hidden=${ele.warningHidden()}>
    <p>
    The width or height of your file exceeds 1024, which may not fit on the screen.
    Press a 'Rescale' button to fix the dimensions while preserving the aspect ratio.
    </p>
    <div>
      <button @click=${() => ele.rescale(1024)}>Rescale to 1024</button>
      <button @click=${() => ele.rescale(512)}>Rescale to 512</button>
      <button @click=${() => ele.rescale(128)}>Rescale to 128</button>
    </div>
  </div>
  <div id=dialog-buttons>
    ${ele.cancelButton()}
    <button class=action ?disabled=${ele.readyToGo()} @click=${ele.go}>Go</button>
  </div>
`;

  private cancelButton = () => {
    if (this.hasCancel()) {
      return html`<button id=cancel @click=${this.cancel}>Cancel</button>`;
    }
    return html``;
  };

  private _state: SkottieConfigState = {
    filename: '',
    lottie: null,
    assetsZip: '',
    assetsFilename: '',
  };

  private _isRatioLocked: boolean = false;
  private _ratio: number = 0;
  private _width: number = DEFAULT_SIZE;
  private _height: number = DEFAULT_SIZE;
  private _fps: number = 0;
  private _backgroundColor: string = BACKGROUND_VALUES.TRANSPARENT;
  private _fileChanged: boolean = false;

  constructor() {
    super(SkottieConfigSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.addEventListener('input', this.inputEvent);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.removeEventListener('input', this.inputEvent);
  }

  get height(): number { return this._height; }

  set height(val: number) {
    this._height = val;
    this._render();
  }

  get state(): SkottieConfigState { return this._state; }

  set state(val: SkottieConfigState) {
    console.log('set state', val);
    this._state = Object.assign({}, val); // make a copy of passed in state.
    this._render();
  }

  get fps(): number { return this._fps; }

  set fps(val: number) {
    this._fps = +val;
    this._render();
  }

  get width(): number { return this._width; }

  set width(val: number) {
    this._width = +val;
    this._render();
  }

  get backgroundColor(): string { return this._backgroundColor; }

  set backgroundColor(val: string) {
    this._backgroundColor = val;
    this._render();
  }

  private hasCancel(): boolean {
    return !!this._state.lottie;
  }

  private readyToGo(): boolean {
    return !this._state.filename && (!!this._state.lottie || !!this._state.assetsZip);
  }

  private onFileChange(e: Event): void {
    const files = (e.target as HTMLInputElement).files!;
    this._fileChanged = true;
    const toLoad = files[0];
    const reader = new FileReader();
    if (toLoad.name.endsWith('.json')) {
      reader.addEventListener('load', () => {
        let parsed: LottieAnimation;
        try {
          parsed = JSON.parse(reader.result as string);
        } catch (error) {
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
        this._state.lottie = null;
        this._state.assetsZip = reader.result as string;
        this._state.filename = toLoad.name;

        this._width = DEFAULT_SIZE;
        this._height = DEFAULT_SIZE;
        this._render();
      });
      reader.addEventListener('error', () => {
        errorMessage(`Failed to load ${toLoad.name}`);
      });
      reader.readAsDataURL(toLoad);
    } else {
      let msg = `Bad file type ${toLoad.name}, only .json and .zip supported`;
      if (!allowZips) {
        msg = `Bad file type ${toLoad.name}, only .json supported`;
      }
      errorMessage(msg);
      this._state.filename = '';
      this._state.lottie = null;
    }
  }

  private onFolderChange(e: Event): void {
    const files = (e.target as HTMLInputElement).files!;
    this._fileChanged = true;
    const toLoad = files[0];
    const reader = new FileReader();
    reader.addEventListener('load', () => {
      this._state.assetsZip = reader.result as string;
      this._state.assetsFilename = toLoad.name;
      this._render();
    });
    reader.addEventListener('error', () => {
      errorMessage(`Failed to load ${toLoad.name}`);
    });
    reader.readAsDataURL(toLoad);
  }

  private toggleRatioLock(e: Event): void {
    e.preventDefault();
    this._isRatioLocked = !this._isRatioLocked;
    this._ratio = this._isRatioLocked ? (this._width / this._height) : 0;
    this._render();
  }

  private onWidthInput(): void {
    if (this._isRatioLocked) {
      this._height = Math.floor(this._width / this._ratio);
      this._render();
    }
  }

  private onHeightInput(): void {
    if (this._isRatioLocked) {
      this._width = Math.floor(this._height * this._ratio);
      this._render();
    }
  }

  private rescale(n: number): void {
    const max = Math.max(this._width, this._height);
    if (max <= n) {
      return;
    }
    this._width = Math.floor((this._width * n) / max);
    this._height = Math.floor((this._height * n) / max);
    this._render();
  }

  private warningHidden(): boolean {
    return this._width <= 1024 && this._width <= 1024;
  }

  private updateState(): void {
    this._width = +$$<HTMLInputElement>('#width', this)!.value;
    this._height = +$$<HTMLInputElement>('#height', this)!.value;
    this._fps = +$$<HTMLInputElement>('#fps', this)!.value;
    this._backgroundColor = $$<HTMLInputElement>('#backgroundColor', this)!.value;
  }

  private go(): void {
    this.updateState();
    this.dispatchEvent(new CustomEvent<SkottieConfigEventDetail>('skottie-selected', {
      detail: {
        state: this._state,
        fileChanged: this._fileChanged,
        width: this._width,
        height: this._height,
        fps: this._fps,
        backgroundColor: this._backgroundColor,
      },
      bubbles: true,
    }));
  }

  private cancel(): void {
    this.dispatchEvent(new CustomEvent('cancelled', { bubbles: true }));
  }

  private inputEvent(): void {
    this.updateState();
    this._render();
  }
}

define('skottie-config-sk', SkottieConfigSk);
