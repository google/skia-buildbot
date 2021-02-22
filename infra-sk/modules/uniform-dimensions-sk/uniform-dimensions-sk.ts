/**
 * @module modules/uniform-dimensions-sk
 * @description <h2><code>uniform-dimensions-sk</code></h2>
 *
 * A control that handles the iDimensions uniform, which reports the x and y
 * dimensions of the canvas.
 *
 * Note that we expect it to always be a float3, even though the z is never
 * used, to be compatible with other shader toy apps.
 *
 * Emits a `dimensions-changed` event when the user has changed the dimensions.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { Uniform, UniformControl } from '../uniform/uniform';

export const dimensionsChangedEventName = 'dimensions-changed';

interface dimension {
  width: number;
  height: number;
}

const choices: dimension[] = [
  { width: 128, height: 128 },
  { width: 256, height: 256 },
  { width: 512, height: 512 },
  { width: 640, height: 480 },
  { width: 640, height: 640 },
  { width: 720, height: 576 },
  { width: 720, height: 720 },
  { width: 800, height: 600 },
  { width: 800, height: 800 },
  { width: 1024, height: 768 },
  { width: 1024, height: 1024 },
  { width: 1440, height: 900 },
  { width: 1440, height: 1440 },
];

const defaultUniform: Uniform = {
  name: 'iResolution',
  columns: 3,
  rows: 1,
  slot: 0,
};

const defaultChoice = 2;

export interface DimensionsChangedEventDetail {
  width: number;
  height: number;
}

export class UniformDimensionsSk extends ElementSk implements UniformControl {
  private _choice: number = defaultChoice;

  private _uniform: Uniform = defaultUniform;

  constructor() {
    super(UniformDimensionsSk.template);
  }


  private static template = (ele: UniformDimensionsSk) => html`
    <select @change=${ele.selectionChanged} size="1">
      ${choices.map((choice, index) => html`
      <option
        value=${index}
        ?selected=${index === ele._choice}
      >
        ${choice.width} x ${choice.height}
      </option>`)}
    </select>
  `;

  applyUniformValues(uniforms: number[]): void {
    uniforms[this._uniform.slot] = choices[this._choice].width;
    uniforms[this._uniform.slot + 1] = choices[this._choice].height;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  restoreUniformValues(uniforms: number[]): void {
    // This is a noop, we don't restore predefined uniform values.
  }

  onRAF(): void {
    // noop.
  }

  needsRAF(): boolean {
    return false;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  attributeChangedCallback(): void{
    this._render();
  }

  private selectionChanged(e: InputEvent) {
    this._choice = +(e.target as HTMLOptionElement).value;
    const detail: DimensionsChangedEventDetail = {
      width: choices[this._choice].width,
      height: choices[this._choice].height,
    };
    this.dispatchEvent(new CustomEvent<DimensionsChangedEventDetail>(dimensionsChangedEventName, { detail: detail, bubbles: true }));
  }

  get uniform(): Uniform {
    return this._uniform;
  }

  set uniform(val: Uniform) {
    if (val.rows !== 1 || val.columns !== 3) {
      throw new Error('A dimensions uniform must be float3.');
    }
    this._uniform = val;
  }

  get choice(): number { return this._choice; }

  set choice(val: number) {
    this._choice = val;
    this._render();
  }
}

define('uniform-dimensions-sk', UniformDimensionsSk);
