/**
 * @module particles-config-sk
 * @description <h2><code>particles-config-sk</code></h2>
 *
 * <p>
 *   A dialog for uploading a Particles JSON file.
 *
 *   Just call show() which returns a Promise that resolves once
 *   the dialog is closed.
 * </p>
 *
 */
import 'elements-sk/styles/buttons';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface ParticlesConfig {
  body: any;
}

export class ParticlesConfigSk extends ElementSk {
  private parsedParticleJSON: any | null = null;

  private fileInput: HTMLInputElement | null = null;

  private dialog: HTMLDialogElement | null = null;

  private resolve: ((p: ParticlesConfig| undefined)=> void) | null = null;

  constructor() {
    super(ParticlesConfigSk.template);
  }

  private static template = (ele: ParticlesConfigSk) => html`
  <dialog>
    <label class=file>Particles file to upload
      <input type=file id=file @change=${ele.onFileChange}/>
    </label>
    <div id=dialog-buttons>
      <button @click=${ele.cancel}>Cancel</button>
      <button class=action ?disabled=${!ele.parsedParticleJSON} @click=${ele.ok}>OK</button>
    </div>
  </dialog>
  `;

  /**
   * show pops up the dialog and returns a Promise
   * that resolves once the user has either clicked
   * OK or Cancel.
   *
   * Upon OK the updated ParticlesConfig will be returned,
   * and upon Cancel 'undefined' will be returned.
   */
  show(config: ParticlesConfig): Promise<ParticlesConfig | undefined> {
    return new Promise<ParticlesConfig | undefined>((resolve) => {
      this.parsedParticleJSON = config.body;
      this.resolve = resolve;
      this._render();
      this.dialog!.showModal();
    });
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.fileInput = $$<HTMLInputElement>('#file', this);
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
      this.parsedParticleJSON = parsed;
      this._render();
    });
    reader.readAsText(this.fileInput!.files![0]);
  }

  private cancel() {
    this.dialog!.close();
    this.resolve!(undefined);
  }

  private ok() {
    this.dialog!.close();
    this.resolve!({
      body: this.parsedParticleJSON,
    });
  }
}

define('particles-config-sk', ParticlesConfigSk);
