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
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import '@vaadin/multi-select-combo-box/theme/lumo/vaadin-multi-select-combo-box.js';

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

  private _checkboxSelected: boolean = false;

  private _splitDisabled: boolean = false;

  private _splitCheckboxDisabled: boolean = false;

  /**
   * Creates an instance of PickerFieldSk.
   * @param label The label for the picker field.
   * @param disableSplit (Optional) Whether to permanently disable/hide the split functionality.
   */
  constructor(label: string, disableSplit: boolean = false) {
    super(PickerFieldSk.template);

    this._label = label;
    this._splitDisabled = disableSplit;
  }

  private static template = (ele: PickerFieldSk) => html`
    <div id="picker-field-${ele.label}">
      <div id="split-by-container">
        <checkbox-sk
          title="Split the chart by attribute."
          name=${ele.label}
          id="split-by"
          label="Split"
          @change=${ele.splitOnValue}
          ?checked=${ele.split}
          ?disabled=${ele._splitCheckboxDisabled}
          ?hidden=${!ele.showSplit}>
        </checkbox-sk>
        <checkbox-sk
          title="Select all values without periods in the name."
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

  /**
   * Handles the 'selected-items-changed' event from the
   * vaadin-multi-select-combo-box.
   * Dispatches a custom 'value-changed' event with the new selected items.
   * @param e The event object.
   */
  private onValueChanged(e: Event) {
    const selectedItems = (e as CustomEvent).detail.value as string[];
    this.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: {
          value: selectedItems, // Forward the array of selected items
          checkboxSelected: this._checkboxSelected,
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
      this._checkboxSelected = true;
      if ((e.currentTarget as HTMLInputElement).checked) {
        // Create a shallow copy to avoid direct mutation issues.
        this.selectedItems = this.options.slice();
      } else {
        // Leave the first item selected.
        this.selectedItems = this.options.slice(0, 1);
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
      this._checkboxSelected = true;
      if ((e.currentTarget as HTMLInputElement).checked) {
        // If all is selected, deselect all and select primary.
        if (this._isAllSelected) {
          this.selectedItems = [];
        }
        // Add all primary options to the current selection, ensuring no duplicates.
        const newSelection = [...new Set([...this.selectedItems, ...this.primaryOptions])];
        this.selectedItems = newSelection;
      } else {
        // Leave the first item selected.
        this.selectedItems = this.options.slice(0, 1);
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

    if (this._splitDisabled) {
      this.disableSplit();
    }
  }

  /**
   * Sets focus on the internal vaadin-multi-select-combo-box element.
   */
  focus() {
    if (this._comboBox !== null) {
      this._comboBox!.focus();
    }
    this._render();
  }

  /**
   * Opens the overlay (dropdown) of the internal
   * vaadin-multi-select-combo-box element.
   */
  openOverlay() {
    if (this._comboBox !== null) {
      this._comboBox!.click();
    }
    this._render();
  }

  /**
   * Disables the picker field and associated checkboxes.
   * Sets the vaadin-multi-select-combo-box to readonly and disables the
   * "Select All", "Split", and "Primary" checkboxes.
   */
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

  /**
   * Enables the picker field and associated checkboxes.
   * Removes the readonly attribute from the vaadin-multi-select-combo-box
   * and enables the "Select All", "Split", and "Primary" checkboxes.
   */
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

  /**
   * Clears the selected value of the combo box and resets the selected items.
   */
  clear() {
    this.focus();
    this.setValue('');
    this.selectedItems = [];
  }

  /**
   * Sets the value of the internal vaadin-multi-select-combo-box element.
   * @param val The string value to set.
   */
  setValue(val: string) {
    this._comboBox!.removeAttribute('value');
    this._comboBox!.setAttribute('value', val);
    this._render();
  }

  /**
   * Enables the split checkbox.
   */
  enableSplit() {
    this._splitCheckboxDisabled = false;
    this._render();
  }

  /**
   * Disables the split checkbox and sets the split property to false.
   */
  disableSplit() {
    this._split = false;
    this._splitCheckboxDisabled = true;
    this._render();
  }

  /**
   * Resets the selected items of the combo box.
   */
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

  /**
   * Returns true if all options are currently selected.
   * @returns True if all options are selected, false otherwise.
   */
  private get _isAllSelected(): boolean {
    if (this._options.length === 0) {
      return false;
    }
    return this.selectedItems.length === this._options.length;
  }

  /**
   * Returns true if all primary options are currently selected.
   * @returns True if all primary options are selected, false otherwise.
   */
  private get _arePrimarySelected(): boolean {
    if (this._primaryOptions.length === 0) {
      return false;
    }
    const show = this._primaryOptions.every((p) => this.selectedItems.includes(p));
    return this._primaryOptions.length === this.selectedItems.length && show;
  }

  /**
   * Gets the index of the picker field.
   * @returns The index of the picker field.
   */
  get index(): number {
    return this._index;
  }

  /**
   * Sets the index of the picker field.
   * @param v The new index value.
   */
  set index(v: number) {
    this._index = v;
  }

  /**
   * Gets the current split state of the picker field.
   * @returns True if the field is split, false otherwise.
   */
  get split(): boolean {
    return this._split;
  }

  /**
   * Sets the split state of the picker field.
   * Updates the checked state of the split checkbox.
   * @param v The new split state.
   */
  set split(v: boolean) {
    this._split = v;
    if (this._splitBox !== null) {
      this._splitBox.checked = v;
    }
    this._render();
  }

  /**
   * Gets the array of available options for the picker field.
   * @returns An array of strings representing the available options.
   */
  get options(): string[] {
    return this._options;
  }

  /**
   * Sets the array of available options for the picker field.
   * Also filters primary options and recalculates the overlay width.
   * @param v The new array of options.
   */
  set options(v: string[]) {
    this._options = v;
    this.primaryOptions = v.filter((option) => !option.includes('.'));
    this.calculateOverlayWidth();
    this._render();
  }

  /**
   * Gets the array of primary options (options without periods) for the
   * picker field.
   * @returns An array of strings representing the primary options.
   */
  get primaryOptions(): string[] {
    return this._primaryOptions;
  }

  /**
   * Sets the array of primary options for the picker field.
   * @param v The new array of primary options.
   */
  set primaryOptions(v: string[]) {
    this._primaryOptions = v;
    this._render();
  }

  /**
   * Gets the label of the picker field.
   * @returns The label of the picker field.
   */
  get label(): string {
    return this._label;
  }

  /**
   * Sets the label of the picker field.
   * @param v The new label value.
   */
  set label(v: string) {
    this._label = v;
    this._render();
  }

  /**
   * Gets the currently selected items of the combo box.
   * @returns An array of strings representing the selected items.
   */
  get selectedItems(): string[] {
    return this._selectedItems;
  }

  /**
   * Sets the selected items of the combo box.
   * @param v The new array of selected items.
   */
  set selectedItems(v: string[]) {
    this._selectedItems = v;
    this._render();
  }

  /**
   * Gets the helper text of the picker field.
   * @returns The helper text of the picker field.
   */
  get helperText(): string {
    return this._helper_text;
  }

  /**
   * Sets the helper text of the picker field.
   * @param v The new helper text value.
   */
  set helperText(v: string) {
    this._helper_text = v;
    this._render();
  }

  /**
   * Returns true if the "Select All" checkbox should be shown.
   * The checkbox is shown if there are more than 2 options and the field is
   * not the first field (index > 0).
   * @returns True if the "Select All" checkbox should be shown, false
   * otherwise.
   */
  get showSelectAll(): boolean {
    return this.options.length > 2 && this.index > 0;
  }

  /**
   * Returns true if the "Split" checkbox should be shown.
   * The checkbox is shown if there are more than 1 selected item and the
   * field is not the first field (index > 0).
   * @returns True if the "Split" checkbox should be shown, false otherwise.
   */
  get showSplit(): boolean {
    if (this._splitDisabled) {
      return false;
    }
    return this.selectedItems.length > 1 && this.index > 0;
  }

  /**
   * Returns true if the "Primary" checkbox should be shown.
   * The checkbox is shown if the number of primary options is different from
   * the total number of options and the field is not the first field
   * (index > 0).
   * @returns True if the "Primary" checkbox should be shown, false otherwise.
   */
  get showPrimary(): boolean {
    return (
      this.primaryOptions.length > 0 &&
      this.primaryOptions.length !== this.options.length &&
      this.index > 0
    );
  }
}

define('picker-field-sk', PickerFieldSk);
