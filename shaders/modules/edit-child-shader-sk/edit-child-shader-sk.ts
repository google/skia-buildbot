/**
 * @module modules/edit-child-shader-sk
 * @description <h2><code>edit-child-shader-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ChildShader } from '../json';
import { childShaderUniformNameRegex } from '../shadernode';

const defaultChildShader: ChildShader = {
  UniformName: 'childShader',
  ScrapHashOrName: '',
};

export class EditChildShaderSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private childShader: ChildShader = defaultChildShader;

  constructor() {
    super(EditChildShaderSk.template);
  }

  private static template = (ele: EditChildShaderSk) => html`<dialog>
    <label>Uniform Name: <input id=uniformName type=text pattern=${childShaderUniformNameRegex.source} .value=${ele.childShader.UniformName}></label>
    <div>
      <button @click=${ele.cancelClick}>Cancel</button>
      <button @click=${ele.okClick}>OK</button>
    </div>
  </dialog>>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
    dialogPolyfill.registerDialog(this.dialog!);
  }

  show(childShader: ChildShader): Promise<ChildShader | undefined> {
    return new Promise((resolve) => {
      this.resolve = resolve;
      this.childShader = Object.assign({}, childShader);
      this._render();
      // eslint-disable-next-line no-unused-expressions
      this.dialog?.showModal();
    });
  }

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private resolve: (value: ChildShader | undefined)=> void = () => {};

  private cancelClick() {
    this.resolve(undefined);
  }

  private okClick() {
    this.childShader.UniformName = $$<HTMLInputElement>('#uniformName', this)!.value;
    this.resolve(this.childShader);
  }
}

define('edit-child-shader-sk', EditChildShaderSk);
