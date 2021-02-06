/**
 * @module modules/uniform-generic-sk
 * @description <h2><code>uniform-generic-sk</code></h2>
 *
 * Generic uniform control to use when no other controls fit better.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'float2x2',
  rows: 1,
  columns: 1,
  slot: 0,
};

export class UniformGenericSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  constructor() {
    super(UniformGenericSk.template);
  }

  private static defaultValue = (ele: UniformGenericSk, rowIndex: number, colIndex: number): string => {
    // Non-square uniforms get a default value of 0.5.
    if (ele._uniform.columns !== ele._uniform.rows) {
      return '0.5';
    }
    // Square uniforms default to the identity matrix.
    if (rowIndex === colIndex) {
      return '1';
    }
    return '0';
  }

  private static row = (ele: UniformGenericSk, rowIndex: number): TemplateResult[] => {
    const ret: TemplateResult[] = [];
    for (let colIndex = 0; colIndex < ele._uniform.columns; colIndex++) {
      ret.push(html`
        <td>
          <input
            type="number"
            value="${UniformGenericSk.defaultValue(ele, rowIndex, colIndex)}"
            id="${ele._uniform.name}_${rowIndex}_${colIndex}"
          >
        </td>`);
    }
    return ret;
  }

  private static rows = (ele: UniformGenericSk): TemplateResult[] => {
    const ret: TemplateResult[] = [];
    for (let i = 0; i < ele._uniform.rows; i++) {
      ret.push(html`<tr> ${UniformGenericSk.row(ele, i)} </tr>`);
    }
    return ret;
  }

  private static template = (ele: UniformGenericSk) => html`
    <table>
      ${UniformGenericSk.rows(ele)}
    </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  /** The description of the uniform. */
  get uniform(): Uniform {
    return this._uniform!;
  }

  set uniform(val: Uniform) {
    this._uniform = val;
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: Float32Array): void {
    for (let col = 0; col < this._uniform.columns; col++) {
      for (let row = 0; row < this._uniform.rows; row++) {
        uniforms[this.uniform.slot + col * this._uniform.rows + row] = $$<HTMLInputElement>(`#${this._uniform.name}_${row}_${col}`, this)!.valueAsNumber;
      }
    }
  }
}

define('uniform-generic-sk', UniformGenericSk);
