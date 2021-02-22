/**
 * @module modules/uniform-slider-sk
 * @description <h2><code>uniform-slider-sk</code></h2>
 *
 * Constructs a single slider for a single float uniform.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'u_time',
  rows: 1,
  columns: 1,
  slot: 0,
};

export class UniformSliderSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  private input: HTMLInputElement | null = null;

  constructor() {
    super(UniformSliderSk.template);
  }

  private static template = (ele: UniformSliderSk) => html`
  <label>
    <input min="0" max="1" step="0.001" type="range" />
    ${ele.uniform.name}
  </label>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.input = $$<HTMLInputElement>('input', this);
  }

  /** The description of the uniform. */
  get uniform(): Uniform {
    return this._uniform!;
  }

  set uniform(val: Uniform) {
    if (val.columns !== 1 || val.rows !== 1) {
      throw new Error(
        'uniform-slider-sk can only work on a uniform of size 1.',
      );
    }
    this._uniform = val;
    this._render();
  }

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: number[]): void {
    uniforms[this.uniform.slot] = this.input!.valueAsNumber;
  }

  restoreUniformValues(uniforms: number[]): void {
      this.input!.valueAsNumber = uniforms[this.uniform.slot];
  }

  onRAF(): void {
    // noop.
  }

  needsRAF(): boolean {
    return false;
  }
}

define('uniform-slider-sk', UniformSliderSk);
