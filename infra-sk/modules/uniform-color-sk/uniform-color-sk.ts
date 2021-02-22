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

/** Converts the uniform value in the range [0, 1] into a two digit hex string. */
export const slotToHex = (uniforms: number[], slot: number): string => {
  const s = Math.floor(0.5 + uniforms[slot] * 255).toString(16);
  if (s.length === 1) {
    return `0${s}`;
  }
  return s;
};

/** Converts the two digit hex into a uniform value in the range [0, 1] */
export const hexToSlot = (hexDigits: string, uniforms: number[], slot: number): void => {
  let colorAsFloat = parseInt(hexDigits, 16) / 255;
  // Truncate to 4 digits of precision.
  colorAsFloat = Math.floor(colorAsFloat * 10000) / 10000;
  uniforms[slot] = colorAsFloat;
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
  applyUniformValues(uniforms: number[]): void {
    // Set all three floats from the color.
    const hex = this.colorInput!.value;
    hexToSlot(hex.slice(1, 3), uniforms, this.uniform.slot);
    hexToSlot(hex.slice(3, 5), uniforms, this.uniform.slot + 1);
    hexToSlot(hex.slice(5, 7), uniforms, this.uniform.slot + 2);

    // Set the alpha channel if present.
    if (this.hasAlphaChannel()) {
      uniforms[this.uniform.slot + 3] = this.alphaInput!.valueAsNumber;
    }
  }

  restoreUniformValues(uniforms: number[]): void {
    const r = slotToHex(uniforms, this.uniform.slot);
    const g = slotToHex(uniforms, this.uniform.slot + 1);
    const b = slotToHex(uniforms, this.uniform.slot + 2);
    this.colorInput!.value = `#${r}${g}${b}`;

    if (this.hasAlphaChannel()) {
      this.alphaInput!.valueAsNumber = uniforms[this.uniform.slot + 3];
    }
  }

  onRAF(): void {
    // noop.
  }

  needsRAF(): boolean {
    return false;
  }

  private hasAlphaChannel(): boolean {
    return this._uniform.columns === 4;
  }
}

define('uniform-color-sk', UniformColorSk);
