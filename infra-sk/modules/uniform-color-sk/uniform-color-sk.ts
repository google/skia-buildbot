/**
 * @module modules/uniform-color-sk
 * @description <h2><code>uniform-color-sk</code></h2>
 *
 * A control for editing a float3 uniform which should be represented as a
 * color.
 *
 * The color uniform values are floats in [0, 1] and are in RGB order.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iColor',
  rows: 1,
  columns: 3,
  slot: 0,
};

export class UniformColorSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  private colorInput: HTMLInputElement | null = null;

  private alphaInput: HTMLInputElement | null = null;

  constructor() {
    super(UniformColorSk.template);
  }

  private static template = (ele: UniformColorSk) => html`
  <label>
    <input id=colorInput value="#808080" type="color" />
    ${ele.uniform.name}
  </label>
  <label class="${ele.hasAlphaChannel() ? '' : 'hidden'}">
    <input id=alphaInput min="0" max="1" step="0.001" type="range" />
    Alpha
  </label>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.colorInput = $$<HTMLInputElement>('#colorInput', this);
    this.alphaInput = $$<HTMLInputElement>('#alphaInput', this);
  }

  /** The description of the uniform. */
  get uniform(): Uniform {
    return this._uniform!;
  }

  set uniform(val: Uniform) {
    if ((val.columns !== 3 && val.columns !== 4) || val.rows !== 1) {
      throw new Error('uniform-color-sk can only work on a uniform of float3 or float4.');
    }
    this._uniform = val;
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: Float32Array): void {
    // Set all three floats from the color.
    const hex = this.colorInput!.value;
    const r = parseInt(hex.slice(1, 3), 16) / 255;
    const g = parseInt(hex.slice(3, 5), 16) / 255;
    const b = parseInt(hex.slice(5, 7), 16) / 255;
    uniforms[this.uniform.slot] = r;
    uniforms[this.uniform.slot + 1] = g;
    uniforms[this.uniform.slot + 2] = b;

    // Set the alpha channel if present.
    if (this.hasAlphaChannel()) {
      uniforms[this.uniform.slot + 3] = this.alphaInput!.valueAsNumber;
    }
  }

  private hasAlphaChannel(): boolean {
    return this._uniform.columns === 4;
  }
}

define('uniform-color-sk', UniformColorSk);
