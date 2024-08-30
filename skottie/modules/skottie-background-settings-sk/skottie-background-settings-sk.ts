/**
 * @module skottie-background-settings-sk
 * @description <h2><code>skottie-background-settings-sk</code></h2>
 *
 * <p>
 *   A component to edit the background color in the sidebar
 * </p>
 *
 *
 * @evt background-change - This event is triggered every time the
 *      user selects a new color or opacity
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-dropdown-sk';
import { DropdownSelectEvent } from '../skottie-dropdown-sk/skottie-dropdown-sk';

type BackgroundMode = 'light' | 'dark' | 'custom';

type ValuePair = {
  color: string;
  opacity: number;
};

const values: Record<BackgroundMode, ValuePair> = {
  light: {
    color: '#FFFFFF',
    opacity: 100,
  },
  dark: {
    color: '#000000',
    opacity: 100,
  },
  custom: {
    color: '#FF0000',
    opacity: 100,
  },
};

export interface SkottieBackgroundSettingsEventDetail {
  color: string;
  opacity: number;
}

export class SkottieBackgroundSettingsSk extends ElementSk {
  private _color: string = values.light.color;

  private _opacity: number = values.light.opacity;

  private _backgroundMode: BackgroundMode = 'light';

  private static template = (ele: SkottieBackgroundSettingsSk) => html`
    <div class="wrapper">
      <skottie-dropdown-sk
        id="background-selector"
        .name="background-selector"
        .options=${[
          {
            id: 'light',
            value: 'Light',
            selected: ele._backgroundMode === 'light',
          },
          {
            id: 'dark',
            value: 'Dark',
            selected: ele._backgroundMode === 'dark',
          },
          {
            id: 'custom',
            value: 'Custom',
            selected: ele._backgroundMode === 'custom',
          },
        ]}
        @select=${ele.backgroundSelectHandler}
        border></skottie-dropdown-sk>
      ${ele.renderColorInput()}
    </div>
  `;

  constructor() {
    super(SkottieBackgroundSettingsSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Query params are not set when `connectedCallback` is called.
    // As a workaround this timeout of 1ms seems to be enough.
    setTimeout(() => {
      const params = new URL(document.location.href).searchParams;
      const color = params.has('bg') ? params.get('bg')! : '';
      if (color) {
        let mode: BackgroundMode;
        if (color === values.light.color) {
          mode = 'light';
        } else if (color === values.dark.color) {
          mode = 'dark';
        } else {
          mode = 'custom';
        }
        this._backgroundMode = mode;
        this._opacity = values[this._backgroundMode].opacity;
        this._color = color;
        this._render();
        this._submit();
      }
    }, 1);
  }

  private renderColorInput(): TemplateResult | null {
    if (this._backgroundMode === 'custom') {
      return html`
        <div class="color-form">
          <label class="color-form--color">
            <input
              type="color"
              value=${this._color}
              @change=${this.onColorChange} />
            <span>${this._color}</span>
          </label>
          <input
            type="number"
            class="color-form--opacity"
            .value=${this._opacity}
            @change=${this.onOpacityChange} />
        </div>
      `;
    }
    return null;
  }

  private _submit(): void {
    this.dispatchEvent(
      new CustomEvent<SkottieBackgroundSettingsEventDetail>(
        'background-change',
        {
          detail: {
            color: this._color,
            opacity: this._opacity,
          },
          bubbles: true,
        }
      )
    );
  }

  private backgroundSelectHandler(ev: CustomEvent<DropdownSelectEvent>): void {
    this._backgroundMode = ev.detail.value as BackgroundMode;
    this._color = values[this._backgroundMode].color;
    this._opacity = values[this._backgroundMode].opacity;
    this._submit();
    this._render();
  }

  private onColorChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._color = input.value;
    this._submit();
    this._render();
  }

  private onOpacityChange(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this._opacity = input.valueAsNumber;
    this._submit();
    this._render();
  }
}

define('skottie-background-settings-sk', SkottieBackgroundSettingsSk);
