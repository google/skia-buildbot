/**
 * @module skottie-dropdown-sk
 * @description <h2><code>skottie-dropdown-sk</code></h2>
 *
 * <p>
 *   A skottie dropdown.
 * </p>
 *
 *
 * @attr options - A list of select options.
 *
 * @attr name - The same of the select.
 *
 * @attr reset - Resets the dropdown after select.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface DropdownOption {
  id: string;
  value: string;
  selected?: boolean;
}

export interface DropdownSelectEvent {
  value: string;
}

export class SkottieDropdownSk extends ElementSk {
  private _options: DropdownOption[] = [];

  private _name: string = '';

  private _hasBorder: boolean = false;

  private _isFull: boolean = false;

  private _reset: boolean = false;

  private static template = (ele: SkottieDropdownSk) => html`
    <select
      name="${ele._name}"
      @change=${ele.onChange}
      class=${ele.buildSelectClass()}>
      ${ele.buildOptions()}
    </select>
  `;

  constructor() {
    super(SkottieDropdownSk.template);
    this._reset = this.hasAttribute('reset');
    this._hasBorder = this.hasAttribute('border');
    this._isFull = this.hasAttribute('full');
  }

  _render() {
    super._render();
    if (this._reset) {
      const select = this.querySelector('select');
      if (select) {
        select.selectedIndex = 0;
      }
    }
  }

  buildSelectClass(): string {
    const classes = ['base'];
    if (this._hasBorder) {
      classes.push('base--border');
    }
    if (this._isFull) {
      classes.push('base--full');
    }
    return classes.join(' ');
  }

  buildOption(option: DropdownOption): TemplateResult {
    return html` <option value=${option.id} ?selected=${option.selected}>
      ${option.value}
    </option>`;
  }

  buildOptions(): TemplateResult[] {
    return this._options.map((option) => this.buildOption(option));
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  onChange(ev: Event): void {
    if ((ev?.target as HTMLSelectElement).value) {
      this.dispatchEvent(
        new CustomEvent<DropdownSelectEvent>('select', {
          detail: {
            value: (ev.target as HTMLSelectElement).value,
          },
          bubbles: true,
        })
      );
    }
    this._render();
  }

  set options(o: DropdownOption[]) {
    this._options = o;
    this._render();
  }

  set name(n: string) {
    this._name = n;
    this._render();
  }
}

define('skottie-dropdown-sk', SkottieDropdownSk);
