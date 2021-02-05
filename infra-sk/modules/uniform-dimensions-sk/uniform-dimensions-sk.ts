/**
 * @module modules/uniform-dimensions-sk
 * @description <h2><code>uniform-dimensions-sk</code></h2>
 *
 * A control that handles the iDimensions uniform, which reports the x and y
 * dimensions of the canvas.
 *
 * Note that we expect it to always be a float3, even though the z is never
 * used, to be compatible with other shader toy apps.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

const defaultUniform: Uniform = {
  name: 'iDimensions',
  columns: 3,
  rows: 1,
  slot: 0,
};

export class UniformDimensionsSk extends ElementSk implements UniformControl {
  private _uniform: Uniform = defaultUniform;

  private static template = (ele: UniformDimensionsSk) =>
    html`<span>${ele.x} x ${ele.y}</span>`;

  constructor() {
    super(UniformDimensionsSk.template);
  }

  applyUniformValues(uniforms: Float32Array): void {
    uniforms[this._uniform.slot] = this.x;
    uniforms[this._uniform.slot + 1] = this.y;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get uniform() {
    return this._uniform;
  }
  set uniform(val) {
    if (val.rows !== 1 || val.columns !== 3) {
      throw new Error('A dimensions uniform must be float3.');
    }
    this._uniform = val;
  }

  static get observedAttributes() {
    return ['x', 'y'];
  }

  get x(): number {
    return +(this.getAttribute('x') || 0);
  }

  set x(val: number) {
    this.setAttribute('x', val.toFixed(0));
  }

  get y(): number {
    return +(this.getAttribute('y') || 0);
  }

  set y(val: number) {
    this.setAttribute('y', val.toFixed(0));
  }

  attributeChangedCallback() {
    this._render();
  }
}

define('uniform-dimensions-sk', UniformDimensionsSk);
