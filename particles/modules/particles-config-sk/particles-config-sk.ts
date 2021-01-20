/**
 * @module particles-config-sk
 * @description <h2><code>particles-config-sk</code></h2>
 *
 * <p>
 *   A dialog for configuring how to render a Particles JSON file.
 * </p>
 *
 */
import 'elements-sk/styles/buttons';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const DEFAULT_SIZE = 800;

export interface ParticlesConfig {
  body: any;
  width: number;
  height: number;
}

export class ParticlesConfigSk extends ElementSk {
  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private body: any | null = null;

  private fileInput: HTMLInputElement | null = null;

  private widthInput: HTMLInputElement | null = null;

  private heightInput: HTMLInputElement | null = null;

  private dialog: HTMLDialogElement | null = null;

  private startValue: ParticlesConfig | null = null;

  private resolve: ((p: ParticlesConfig)=> void) | null = null;

  constructor() {
    super(ParticlesConfigSk.template);
  }

  private static template = (ele: ParticlesConfigSk) => html`
  <dialog>
    <label class=file>Particles file to upload
      <input type=file id=file @change=${ele.onFileChange}/>
    </label>
    <label>
      <input
       id=width
       type=number
       .value=${ele.width.toFixed(0)}
       @input=${() => { ele.width = +ele.widthInput!.value; }}
      /> Width (px)
    </label>
    <label>
      <input
        id=height
        type=number
        .value=${ele.height.toFixed(0)}
        @input=${() => { ele.height = +ele.heightInput!.value; }}
      /> Height (px)
    </label>
    <div id=dialog-buttons>
      <button @click=${ele.cancel}>Cancel</button>
      <button class=action ?disabled=${!ele.body} @click=${ele.ok}>OK</button>
    </div>
</dialog>
  `;

  show(config: ParticlesConfig): Promise<ParticlesConfig> {
    this.startValue = Object.assign({}, config);
    return new Promise<ParticlesConfig>((resolve) => {
      this.width = config.width;
      this.height = config.height;
      this.body = config.body;
      this.resolve = resolve;
      this._render();
      this.dialog!.showModal();
    });
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.fileInput = $$<HTMLInputElement>('#file', this);
    this.widthInput = $$<HTMLInputElement>('#width', this);
    this.heightInput = $$<HTMLInputElement>('#height', this);
    this.dialog = $$<HTMLDialogElement>('dialog', this);
  }

  private onFileChange() {
    const reader = new FileReader();
    reader.addEventListener('load', () => {
      const body = reader.result as string;
      let parsed: any = {};
      try {
        parsed = JSON.parse(body);
      } catch (error) {
        errorMessage(`Not a valid JSON file: ${error}`);
        return;
      }
      this.body = parsed;
      this.width = parsed.w || DEFAULT_SIZE;
      this.height = parsed.h || DEFAULT_SIZE;
      this._render();
    });
    reader.readAsText(this.fileInput!.files![0]);
  }

  private cancel() {
    this.dialog!.close();
    this.resolve!(this.startValue!);
  }

  private ok() {
    this.dialog!.close();
    this.resolve!({
      body: this.body,
      width: this.width,
      height: this.height,
    });
  }
}

define('particles-config-sk', ParticlesConfigSk);
