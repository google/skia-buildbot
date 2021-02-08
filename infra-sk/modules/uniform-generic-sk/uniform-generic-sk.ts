/**
 * @module modules/uniform-generic-sk
 * @description <h2><code>uniform-generic-sk</code></h2>
 *
 * Generic uniform control to use when no other controls fit better.
 *
 * Simply displays number input controls in a table.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iCoord',
  rows: 1,
  columns: 2,
  slot: 0,
};

export class UniformGenericSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  constructor() {
    super(UniformGenericSk.template);
  }

  private static defaultValue = (ele: UniformGenericSk, row: number, col: number): string => {
    // Non-square uniforms (rows != columns) get a default value of 0.5.
    if (ele._uniform.columns !== ele._uniform.rows) {
      return '0.5';
    }
    // Square uniforms (rows === columns) default to the identity matrix.
    if (row === col) {
      return '1';
    }
    return '0';
  }

  private static row = (ele: UniformGenericSk, row: number): TemplateResult[] => {
    const ret: TemplateResult[] = [];
    for (let col = 0; col < ele._uniform.columns; col++) {
      ret.push(html`
        <td>
          <input
            value="${UniformGenericSk.defaultValue(ele, row, col)}"
            id="${ele._uniform.name}_${row}_${col}"
          >
        </td>`);
    }
    return ret;
  }

  private static rows = (ele: UniformGenericSk): TemplateResult[] => {
    const ret: TemplateResult[] = [];
    for (let row = 0; row < ele._uniform.rows; row++) {
      ret.push(html`<tr> ${UniformGenericSk.row(ele, row)} </tr>`);
    }
    return ret;
  }

  private static template = (ele: UniformGenericSk) => html`
    <div>
      <table>
        ${UniformGenericSk.rows(ele)}
      </table>
      <span>${ele._uniform.name}</span>
    </div>
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
  applyUniformValues(uniforms: number[]): void {
    for (let col = 0; col < this._uniform.columns; col++) {
      for (let row = 0; row < this._uniform.rows; row++) {
        uniforms[this.uniform.slot + col * this._uniform.rows + row] = +$$<HTMLInputElement>(`#${this._uniform.name}_${row}_${col}`, this)!.value;
      }
    }
  }

  restoreUniformValues(uniforms: number[]): void {
    for (let col = 0; col < this._uniform.columns; col++) {
      for (let row = 0; row < this._uniform.rows; row++) {
        $$<HTMLInputElement>(`#${this._uniform.name}_${row}_${col}`, this)!.value = uniforms[this.uniform.slot + col * this._uniform.rows + row].toString();
      }
    }
  }
}

define('uniform-generic-sk', UniformGenericSk);
