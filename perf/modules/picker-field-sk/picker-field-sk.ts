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
import '@vaadin/multi-select-combo-box/theme/lumo/vaadin-multi-select-combo-box.js';
import '@vaadin/combo-box/theme/lumo/vaadin-combo-box.js';
import '@vaadin/multi-select-combo-box/theme/lumo/vaadin-multi-select-combo-box.js';

export interface SplitChartSelectionEventDetails {
  attribute: string;
}

export class PickerFieldSk extends ElementSk {
  private _label: string = '';

  private _helper_text: string = '';

  private _options: string[] = [];

  private _comboBox: HTMLElement | null = null;

  private _splitBox: HTMLElement | null = null;

  private _selectedItems: string[] = [];

  private _splitBy: boolean = false;

  constructor(label: string) {
    super(PickerFieldSk.template);
    this._label = label;
  }

  private static template = (ele: PickerFieldSk) => html`
    <div id="picker-field">
      <div id="split-by-container">
        <checkbox-sk
          title="Splits the chart by attribute."
          name=${ele.label}
          label="Split"
          @change=${ele.splitOnValue}
          ?checked=${ele._splitBy}
          ?hidden=${ele.selectedItems.length < 2}>
        </checkbox-sk>
      </div>
      <vaadin-multi-select-combo-box
        @selected-items-changed=${ele.onValueChanged}
        helper-text="${ele.helperText}"
        label="${ele.label}"
        .items=${ele.options}
        .selectedItems=${ele.selectedItems}
        auto-expand-horizontally
        selected-items-on-top>
      </vaadin-multi-select-combo-box>
    </div>
  `;

  private onValueChanged(e: Event) {
    const selectedItems = (e as CustomEvent).detail.value as string[];

    this.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: {
          value: selectedItems, // Forward the array of selected items
        },
      })
    );
  }

  private splitOnValue() {
    this._splitBy = !this._splitBy;
    this.dispatchEvent(
      new CustomEvent('split-by-changed', {
        detail: {
          param: this.label,
          split: this._splitBy,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._comboBox = this.querySelector('vaadin-multi-select-combo-box');
    this._splitBox = this.querySelector('checkbox-sk');
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
    if (this._comboBox !== null) {
      this._comboBox!.setAttribute('readonly', '');
      this._render();
    }
  }

  enable() {
    if (this._comboBox !== null) {
      this._comboBox!.removeAttribute('opened');
      this._comboBox!.removeAttribute('readonly');
      this._render();
    }
  }

  clear() {
    this.focus();
    this.setValue('');
    this.selectedItems = [];
  }

  setValue(val: string) {
    this._comboBox!.removeAttribute('value');
    this._comboBox!.setAttribute('value', val);
    this._render();
  }

  enableSplit() {
    if (this._splitBox !== null) {
      this._splitBox.removeAttribute('disabled');
    }
    this._render();
  }

  disableSplit() {
    if (this._splitBox !== null) {
      this._splitBy = false;
      this._splitBox.setAttribute('disabled', '');
    }
    this._render();
  }

  setSplit(val: boolean) {
    this._splitBy = val;
    this._render();
  }

  reset() {
    this.selectedItems = [];
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
    if (this._comboBox !== null) {
      this._comboBox!.style.setProperty(
        '--vaadin-multi-select-combo-box-overlay-width',
        `${maxLength + 5}ch`
      );
    }
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

  get selectedItems(): string[] {
    return this._selectedItems;
  }

  set selectedItems(v: string[]) {
    this._selectedItems = v;
    this._render();
  }

  get helperText(): string {
    return this._helper_text;
  }

  set helperText(v: string) {
    this._helper_text = v;
    this._render();
  }
}

define('picker-field-sk', PickerFieldSk);
