/**
 * @module skottie-file-form-sk
 * @description <h2><code>skottie-file-form-sk</code></h2>
 *
 * <p>
 *   A component to upload lottie files in the sidebar
 * </p>
 *
 *
 * @evt files-selected - This event is generated when the user presses Apply.
 *      The updated state is submitted with all files attached to the form.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/icons/visibility-icon-sk';
import '../../../elements-sk/modules/icons/visibility-off-icon-sk';
import '../skottie-button-sk';
import { LottieAnimation } from '../types';
import { SoundMap } from '../audio';
import { $$ } from '../../../infra-sk/modules/dom';

export interface SkottieFileState {
  assets?: Record<string, ArrayBuffer>;
  filename: string;
  lottie: LottieAnimation | null;
  assetsZip: string;
  assetsFilename: string;
  w?: number;
  h?: number;
  soundMap?: SoundMap;
}

export type SkottieFilesEventDetail = SkottieFileState;

const allowZips =
  window.location.hostname === 'skottie-internal.skia.org' ||
  window.location.hostname === 'localhost';

const defaultState: SkottieFileState = {
  filename: '',
  lottie: null,
  assetsZip: '',
  assetsFilename: '',
};

export class SkottieFileFormSk extends ElementSk {
  private _state: SkottieFileState = {
    ...defaultState,
  };

  private static template = (ele: SkottieFileFormSk) => html`
    <div class="wrapper">${ele.renderForm()} ${ele.renderSubmit()}</div>
  `;

  constructor() {
    super(SkottieFileFormSk.template);
  }

  renderForm(): TemplateResult {
    return html`
      <form class="upload-file" id="upload-files">
        <label class="upload-file--label">
          ${this._state.filename || '+ Upload Lottie file'}
          <input
            type="file"
            name="file"
            id="upload-file"
            class="upload-file--input"
            @change=${this.onFileChange} />
        </label>
        <label class="upload-file--label">
          ${this._state.assetsFilename || '+ Optional Asset Folder (.zip)'}
          <input
            ?hidden=${!allowZips}
            type="file"
            name="file"
            id="upload-assets"
            class="upload-file--input"
            @change=${this.onFolderChange} />
        </label>
      </form>
    `;
  }

  renderSubmit(): TemplateResult | null {
    if (!this._state.lottie) {
      return null;
    }
    return html`
      <div class="toolbar">
        <skottie-button-sk
          type="plain"
          @select=${this.clear}
          .content=${'Clear'}>
        </skottie-button-sk>
        <skottie-button-sk
          type="filled"
          @select=${this.apply}
          .content=${'Upload'}>
        </skottie-button-sk>
      </div>
    `;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private onFileChange(e: Event): void {
    const files = (e.target as HTMLInputElement).files!;
    const toLoad = files[0];
    const reader = new FileReader();
    if (toLoad.name.endsWith('.json')) {
      reader.addEventListener('load', () => {
        let parsed: LottieAnimation;
        try {
          parsed = JSON.parse(reader.result as string) as LottieAnimation;
        } catch (error) {
          errorMessage(`Not a valid JSON file: ${error}`);
          return;
        }
        this._state.lottie = parsed;
        this._state.filename = toLoad.name;
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

  private apply(): void {
    this.dispatchEvent(
      new CustomEvent<SkottieFilesEventDetail>('files-selected', {
        detail: this._state,
        bubbles: true,
      })
    );
    this.clear();
  }

  private clear(): void {
    this._state = { ...defaultState };
    const form = $$<HTMLFormElement>('#upload-files');
    form?.reset();
    this._render();
  }
}

define('skottie-file-form-sk', SkottieFileFormSk);
