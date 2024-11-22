/**
 * @module skottie-vec2-input-sk
 * @description <h2><code>skottie-vec2-input-sk</code></h2>
 *
 * <p>
 *   A skottie vec2 input to manage 2 related values
 * </p>
 *
 *
 * @attr label - A string to define the pair
 *
 * @prop label This mirrors the text attribute.
 *
 * @attr x - A number representing the first value
 *
 * @prop x This mirrors the type attribute.
 *
 * @attr y - A number representing the second value
 *
 * @prop y This mirrors the type attribute.
 *
 * @evt value-change - This event is triggered every time either x or y change.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface SkottieVec2EventDetail {
  label: string;
  x: number;
  y: number;
}

export class SkottieVec2InputSk extends ElementSk {
  private _label: string = '';

  private _x: number = 0;

  private _y: number = 0;

  private static template = (ele: SkottieVec2InputSk) => html`
    <div class="slot--vec2">
      <span class="slotID">${ele._label}</span>
      <div class="text-box text-box__left">
        <input
          type="number"
          class="text-box--input"
          id="file-settings-width"
          @change=${ele.onXChange}
          value=${ele._x}
          required />
      </div>
      <div class="text-box text-box__right">
        <input
          type="number"
          class="text-box--input"
          id="file-settings-height"
          @change=${ele.onYChange}
          value=${ele._y}
          required />
      </div>
    </div>
  `;

  constructor() {
    super(SkottieVec2InputSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private onXChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._x = input.valueAsNumber;
    this.submit();
    this._render();
  }

  private onYChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._y = input.valueAsNumber;
    this.submit();
    this._render();
  }

  private submit(): void {
    this.dispatchEvent(
      new CustomEvent<SkottieVec2EventDetail>('vec2-change', {
        detail: {
          label: this._label,
          x: this._x,
          y: this._y,
        },
        bubbles: true,
      })
    );
  }

  set label(value: string) {
    this._label = value;
    this._render();
  }

  set x(value: number) {
    this._x = value;
    this._render();
  }

  set y(value: number) {
    this._y = value;
    this._render();
  }
}

define('skottie-vec2-input-sk', SkottieVec2InputSk);
