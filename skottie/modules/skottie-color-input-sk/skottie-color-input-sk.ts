/**
 * @module skottie-color-input-sk
 * @description <h2><code>skottie-color-input-sk</code></h2>
 *
 * <p>
 *   A skottie color input to manage color and opacity.
 * </p>
 *
 *
 * @attr color - A string, defined as an hexadecimal RGB.
 *
 * @prop color This mirrors the text attribute.
 *
 * @attr opacity - A number that ranges from 0 to 100
 *
 * @prop opacity This mirrors the type attribute.
 *
 * @evt color-change - This event is triggered every time the opacity
 *         or the color is changed.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface SkottieColorEventDetail {
  color: string;
  opacity: number;
}

export class SkottieColorInputSk extends ElementSk {
  private _color: string = '#FF0000';

  private _opacity: number = 100;

  private _withOpacity: boolean = true;

  private static template = (ele: SkottieColorInputSk) => html`
    <div class="wrapper">
      <label
        class="wrapper--color ${ele._withOpacity
          ? ' wrapper--color__withOpacity'
          : ''}">
        <input
          type="color"
          value=${ele._color}
          @change=${ele.onColorChange}
          class="wrapper--color--input" />
        <span>${ele._color}</span>
      </label>
      ${ele.renderOpacity()}
    </div>
  `;

  constructor() {
    super(SkottieColorInputSk.template);
    this._withOpacity = this.hasAttribute('withOpacity');
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private renderOpacity(): TemplateResult | null {
    if (this._withOpacity) {
      return html`
        <input
          type="number"
          class="wrapper--opacity"
          .value=${this._opacity}
          @change=${this.onOpacityChange} />
      `;
    }
    return null;
  }

  private onColorChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._color = input.value;
    this.submit();
    this._render();
  }

  private onOpacityChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._opacity = input.valueAsNumber;
    this.submit();
    this._render();
  }

  private submit(): void {
    this.dispatchEvent(
      new CustomEvent<SkottieColorEventDetail>('color-change', {
        detail: {
          color: this._color,
          opacity: this._opacity,
        },
        bubbles: true,
      })
    );
  }

  set color(value: string) {
    this._color = value;
    this._render();
  }

  set opacity(value: number) {
    this._opacity = value;
    this._render();
  }
}

define('skottie-color-input-sk', SkottieColorInputSk);
