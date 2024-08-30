/**
 * @module modules/picker-field-sk
 * @description <h2><code>picker-field-sk</code></h2>
 *
 * A stylized text field that let's the user pick a value from a pre-selected
 * list of valid options. Specifically, designed to be used by the new Test
 * Picker component, but may be reused for other purposes.
 *
 * @evt value-changed - This event gets triggered whenever a text field value
 * is selected. Clearing the text field counts as selecting a value. The
 * value will be available in event.detail.value.
 *
 * @attr {string} label - The label to be used on top of the text field and as
 * a placeholder in the text field.
 *
 * @attr {string[]} options - A valid selection of options that'll be displayed
 * as options to the user in the dropdown menu.
 *
 * @example
 * <picker-field-sk
 *  .label="benchmark"
 *  .options=${["Speedometer2", "Jetstream2"]}
 * >
 * </picker-field-sk>
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '@vaadin/combo-box/theme/lumo/vaadin-combo-box.js';

export class PickerFieldSk extends ElementSk {
  private _label: string = '';

  private _options: string[] = [];

  private _comboBox: HTMLElement | null = null;

  constructor(label: string) {
    super(PickerFieldSk.template);
    this._label = label;
  }

  private static template = (ele: PickerFieldSk) => html`
    <div>
      <vaadin-combo-box
        label="${ele.label}"
        placeholder="${ele.label}"
        .items=${ele.options}
        theme="small"
        clear-button-visible
        autoselect>
      </vaadin-combo-box>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._comboBox = this.querySelector('vaadin-combo-box');
  }

  focus() {
    this._comboBox!.focus();
    this._render();
  }

  openOverlay() {
    this._comboBox!.click();
    this._render();
  }

  disable() {
    this._comboBox!.setAttribute('readonly', '');
    this._render();
  }

  enable() {
    this._comboBox!.removeAttribute('readonly');
    this._render();
  }

  clear() {
    this.setValue('');
  }

  setValue(val: string) {
    this._comboBox!.removeAttribute('value');
    this._comboBox!.setAttribute('value', val);
    this._render();
  }

  /**
   * Set the overlay width based on the ComboBox's options.
   *
   * Calculate the longest string from the options. Then set the
   * width property to length + 5 (padding) "ch" (character width unit).
   *
   */
  private calculateOverlayWidth() {
    let maxLength = 0;
    this._options.forEach((option) => {
      if (option.length > maxLength) {
        maxLength = option.length;
      }
    });
    this._comboBox!.style.setProperty(
      '--vaadin-combo-box-overlay-width',
      `${maxLength + 5}ch`
    );
  }

  get options(): string[] {
    return this._options;
  }

  set options(v: string[]) {
    this._options = v;
    this.calculateOverlayWidth();
    this._render();
  }

  get label(): string {
    return this._label;
  }

  set label(v: string) {
    this._label = v;
    this._render();
  }

  get comboBox(): HTMLElement | null {
    return this._comboBox;
  }
}

define('picker-field-sk', PickerFieldSk);
