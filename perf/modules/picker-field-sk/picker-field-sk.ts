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
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import { MultiSelectComboBox } from '@vaadin/multi-select-combo-box';

export interface SplitChartSelectionEventDetails {
  attribute: string;
}

export class PickerFieldSk extends ElementSk {
  private _label: string = '';

  private _helper_text: string = '';

  private _options: string[] = [];

  private _comboBox: HTMLElement | null = null;

  private _splitBox: CheckOrRadio | null = null;

  private _allSelected: CheckOrRadio | null = null;

  private _primarySelected: CheckOrRadio | null = null;

  private _selectedItems: string[] = [];

  private _primaryOptions: string[] = [];

  private _split: boolean = false;

  private _index: number = 0;

  constructor(label: string) {
    super(PickerFieldSk.template);

    this._label = label;
  }

  private static template = (ele: PickerFieldSk) => html`
    <div id="picker-field-${ele.label}">
      <div id="split-by-container">
        <checkbox-sk
          title="Splits the chart by attribute."
          name=${ele.label}
          id="split-by"
          label="Split"
          @change=${ele.splitOnValue}
          ?checked=${ele.split}
          ?hidden=${!ele.showSplit}>
        </checkbox-sk>
        <checkbox-sk
          title="Select Primary (without periods)."
          name=${ele.label}
          id="select-primary"
          label="Primary"
          @change=${ele.selectPrimary}
          ?checked=${ele._arePrimarySelected}
          ?hidden=${!ele.showPrimary}>
        </checkbox-sk>
        <checkbox-sk
          title="Select All"
          name=${ele.label}
          id="select-all"
          label="All"
          @change=${ele.selectAll}
          ?checked=${ele._isAllSelected}
          ?hidden=${!ele.showSelectAll}>
        </checkbox-sk>
      </div>
      <vaadin-multi-select-combo-box
        auto-expand-vertically
        label=${ele.label}
        .items=${ele.options}
        .selectedItems=${ele.selectedItems}
        @selected-items-changed=${ele.onValueChanged}
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
        bubbles: true,
        composed: true,
      })
    );
  }

  /**
   * Handles the change event for the "Split By" checkbox.
   * It updates the `_splitBy` property and dispatches a custom event
   * to notify that the split option has changed.
   *
   * @param e - The event triggered by the checkbox change.
   */
  private splitOnValue(e: Event) {
    this._split = (e.currentTarget as HTMLInputElement).checked;
    this.dispatchEvent(
      new CustomEvent('split-by-changed', {
        detail: {
          param: this.label,
          split: this._split,
        },
        bubbles: true,
        composed: true,
      })
    );
  }

  /**
   * Selects all options if the "Select All" checkbox is checked.
   * If it is unchecked, it clears the selection.
   *
   * @param e - The event triggered by the checkbox change.
   */
  private selectAll(e: Event) {
    if (this._allSelected) {
      if ((e.currentTarget as HTMLInputElement).checked) {
        // Create a shallow copy to avoid direct mutation issues.
        this.selectedItems = this.options.slice();
        (this._comboBox as MultiSelectComboBox)!.selectedItems = this.selectedItems;
      } else {
        // Leave the first item selected.
        this.selectedItems = this.options.slice(0, 1);
        (this._comboBox as MultiSelectComboBox)!.selectedItems = this.selectedItems;
      }
    }
  }

  /**
   * Selects or unselects primary options based on the checkbox state.
   * If checked, it adds all primary options to the current selection.
   * If unchecked, it removes all primary options from the current selection.
   */
  private selectPrimary(e: Event) {
    if (this._primarySelected) {
      if ((e.currentTarget as HTMLInputElement).checked) {
        // If all is selected, deselect all and select primary.
        if (this._isAllSelected) {
          this.selectedItems = [];
        }
        // Add all primary options to the current selection, ensuring no duplicates.
        const newSelection = [...new Set([...this.selectedItems, ...this.primaryOptions])];
        this.selectedItems = newSelection;
        (this._comboBox as MultiSelectComboBox)!.selectedItems = this.selectedItems;
      } else {
        // Leave the first item selected.
        this.selectedItems = this.options.slice(0, 1);
        (this._comboBox as MultiSelectComboBox)!.selectedItems = this.selectedItems;
      }
    }
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._comboBox = this.querySelector('vaadin-multi-select-combo-box');
    this._splitBox = this.querySelector('checkbox-sk#split-by');
    this._allSelected = this.querySelector('checkbox-sk#select-all');
    this._primarySelected = this.querySelector('checkbox-sk#select-primary');
  }

  focus() {
    if (this._comboBox !== null) {
      this._comboBox!.focus();
    }
    this._render();
  }

  openOverlay() {
    if (this._comboBox !== null) {
      this._comboBox!.click();
    }
    this._render();
  }

  disable() {
    if (this._comboBox !== null) {
      this._comboBox!.setAttribute('readonly', '');
      if (this._allSelected !== null) {
        this._allSelected.disabled = true;
      }
      if (this._splitBox !== null) {
        this._splitBox.disabled = true;
      }
      if (this._primarySelected !== null) {
        this._primarySelected.disabled = true;
      }
      this._render();
    }
  }

  enable() {
    if (this._comboBox !== null) {
      this._comboBox!.removeAttribute('opened');
      this._comboBox!.removeAttribute('readonly');
      if (this._allSelected !== null) {
        this._allSelected.disabled = false;
      }
      if (this._splitBox !== null) {
        this._splitBox.disabled = false;
      }
      if (this._primarySelected !== null) {
        this._primarySelected.disabled = false;
      }
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
      this._split = false;
      this._splitBox.setAttribute('disabled', '');
    }
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
    this.options.forEach((option) => {
      if (option.length > maxLength) {
        maxLength = option.length;
      }
    });
    const width = `${maxLength + 5}ch`;
    if (this._comboBox !== null) {
      this._comboBox!.style.setProperty('--vaadin-multi-select-combo-box-overlay-width', width);
      this._comboBox!.style.width = width;
    }
  }

  private get _isAllSelected(): boolean {
    if (this._options.length === 0) {
      return false;
    }
    return this.selectedItems.length === this._options.length;
  }

  private get _arePrimarySelected(): boolean {
    if (this._primaryOptions.length === 0) {
      return false;
    }
    const show = this._primaryOptions.every((p) => this.selectedItems.includes(p));
    return this._primaryOptions.length === this.selectedItems.length && show;
  }

  get index(): number {
    return this._index;
  }

  set index(v: number) {
    this._index = v;
  }

  get split(): boolean {
    return this._split;
  }

  set split(v: boolean) {
    this._split = v;
    if (this._splitBox !== null) {
      this._splitBox.checked = v;
    }
    this._render();
  }

  get options(): string[] {
    return this._options;
  }

  set options(v: string[]) {
    this._options = v;
    this.primaryOptions = v.filter((option) => !option.includes('.'));
    this.calculateOverlayWidth();
    this._render();
  }

  get primaryOptions(): string[] {
    return this._primaryOptions;
  }

  set primaryOptions(v: string[]) {
    this._primaryOptions = v;
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

  get showSelectAll(): boolean {
    return this.options.length > 2 && this.index > 0;
  }

  get showSplit(): boolean {
    return this.selectedItems.length > 1 && this.index > 0;
  }

  get showPrimary(): boolean {
    return (
      this.primaryOptions.length > 0 &&
      this.primaryOptions.length !== this.options.length &&
      this.index > 0
    );
  }
}

define('picker-field-sk', PickerFieldSk);
