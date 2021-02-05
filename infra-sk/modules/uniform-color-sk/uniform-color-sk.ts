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
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'u_color',
  rows: 3,
  columns: 1,
  slot: 0,
};

export class UniformColorSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  private input: HTMLInputElement | null = null;

  private static template = (ele: UniformColorSk) => html` <label>
    <input value="#808080" type="color" />
    ${ele.uniform.name}
  </label>`;

  constructor() {
    super(UniformColorSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.input = $$<HTMLInputElement>('input', this);
  }

  /** The description of the uniform. */
  get uniform(): Uniform {
    return this._uniform!;
  }

  set uniform(val: Uniform) {
    // TODO(jcgregorio) Handle the case of a float4, i.e. there is an alpha channel.
    if (val.rows !== 3 || val.columns !== 1) {
      throw new Error('uniform-color-sk can only work on a uniform of float3.');
    }
    this._uniform = val;
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: Float32Array): void {
    // Set all three floats from the color.
    const hex = this.input!.value;
    const r = parseInt(hex.slice(1, 3), 16) / 255;
    const g = parseInt(hex.slice(3, 5), 16) / 255;
    const b = parseInt(hex.slice(5, 7), 16) / 255;
    uniforms[this.uniform.slot] = r;
    uniforms[this.uniform.slot + 1] = g;
    uniforms[this.uniform.slot + 2] = b;
  }
}

define('uniform-color-sk', UniformColorSk);
