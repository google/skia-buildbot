/**
 * @module modules/edit-child-shader-sk
 * @description <h2><code>edit-child-shader-sk</code></h2>
 *
 * Pops up a dialog to edit a ChildShader's name.
 *
 * May be expanded in the future to also edit the ScrapHashOrName.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ChildShader } from '../json';
import { childShaderUniformNameRegex } from '../shadernode';
import 'elements-sk/styles/buttons';

const defaultChildShader: ChildShader = {
  UniformName: 'childShader',
  ScrapHashOrName: '',
};

export class EditChildShaderSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private childShader: ChildShader = defaultChildShader;

  private input: HTMLInputElement | null = null;

  private resolve: ((value: ChildShader | undefined)=> void) | null = null;

  constructor() {
    super(EditChildShaderSk.template);
  }

  private static template = (ele: EditChildShaderSk) => html`<dialog>
    <label>Uniform Name: <input @input=${() => ele._render()} id=uniformName type=text pattern=${childShaderUniformNameRegex.source} .value=${ele.childShader.UniformName}></label>
    <div class=controls>
      <button @click=${ele.cancelClick} id=cancel>Cancel</button>
      <button @click=${ele.okClick} id=ok ?disabled=${ele.input?.validity.patternMismatch}>OK</button>
    </div>
    <span class=error ?hidden=${!ele.input?.validity.patternMismatch} >Not a valid uniform name.</span>
  </dialog>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$<HTMLDialogElement>('dialog', this);
    this.dialog!.addEventListener('close', () => this.dialogClosed());
    this.input = $$<HTMLInputElement>('input', this);
    dialogPolyfill.registerDialog(this.dialog!);
  }

  /**
   * Displays the dialog to edit the ChildShader uniform name.
   *
   * Will resolve to an updated copy of the ChildShader, or 'undefined'
   * if the user presses cancel.
   */
  show(childShader: ChildShader): Promise<ChildShader | undefined> {
    return new Promise((resolve) => {
      this.resolve = resolve;
      this.childShader = Object.assign({}, childShader);
      this._render();
      // eslint-disable-next-line no-unused-expressions
      this.dialog?.showModal();
    });
  }

  private dialogClosed() {
    if (!this.resolve) {
      return;
    }
    this.resolve(undefined);
    this.resolve = null;
  }

  private cancelClick() {
    if (!this.resolve) {
      return;
    }
    this.resolve(undefined);
    this.dialog!.close();
    this.resolve = null;
  }

  private okClick() {
    if (!this.resolve) {
      return;
    }
    this.childShader.UniformName = $$<HTMLInputElement>('#uniformName', this)!.value;
    this.resolve(this.childShader);
    this.dialog!.close();
    this.resolve = null;
  }
}

define('edit-child-shader-sk', EditChildShaderSk);
