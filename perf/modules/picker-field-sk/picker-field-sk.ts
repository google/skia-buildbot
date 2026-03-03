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
import { html, LitElement, PropertyValues } from 'lit';
import { property, state, customElement, query } from 'lit/decorators.js';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import '@vaadin/multi-select-combo-box/theme/lumo/vaadin-multi-select-combo-box.js';
import { MultiSelectComboBox } from '@vaadin/multi-select-combo-box';

export interface SplitChartSelectionEventDetails {
  attribute: string;
}

@customElement('picker-field-sk')
export class PickerFieldSk extends LitElement {
  @property({ type: String })
  label: string = '';

  @property({ type: String, attribute: 'helper-text' })
  helperText: string = '';

  @property({ type: Array, attribute: false })
  options: string[] = [];

  @property({ type: Number })
  index: number = 0;

  @property({ type: Boolean })
  split: boolean = false;

  @property({ type: Array, attribute: false })
  selectedItems: string[] = [];

  // Used to control split availability from constructor/parent
  @property({ type: Boolean, attribute: 'split-disabled' })
  splitDisabled: boolean = false;

  @property({ type: Boolean, reflect: true })
  disabled: boolean = false;

  @state()
  private _checkboxSelected: boolean = false;

  @state()
  private _primaryOptions: string[] = [];

  @state()
  private _overlayWidth: string = '5ch';

  @query('vaadin-multi-select-combo-box')
  private _comboBox!: MultiSelectComboBox;

  @query('checkbox-sk#split-by')
  private _splitBox!: CheckOrRadio;

  @query('checkbox-sk#select-all')
  private _allSelected!: CheckOrRadio;

  @query('checkbox-sk#select-primary')
  private _primarySelected!: CheckOrRadio;

  private _splitCheckboxDisabled: boolean = false;

  /**
   * Creates an instance of PickerFieldSk.
   * @param label The label for the picker field.
   * @param disableSplit (Optional) Whether to permanently disable/hide the split functionality.
   */
  constructor(label: string = '', disableSplit: boolean = false) {
    super();
    this.label = label;
    this.splitDisabled = disableSplit;
  }

  createRenderRoot() {
    return this;
  }

  willUpdate(changedProperties: PropertyValues): void {
    if (changedProperties.has('options')) {
      this._primaryOptions = this.options.filter((option) => !option.includes('.'));

      // Calculate overlay width
      let maxLength = 0;
      this.options.forEach((option) => {
        if (option.length > maxLength) {
          maxLength = option.length;
        }
      });
      this._overlayWidth = `${maxLength + 5}ch`;
    }

    // Propagate disabled state to underlying logic if needed.
    if (changedProperties.has('splitDisabled') && this.splitDisabled) {
      if (this.split) {
        this.split = false;
      }
    }
  }

  render() {
    return html`
      <div id="picker-field-${this.label}">
        <div id="split-by-container">
          <checkbox-sk
            title="Split the chart by attribute."
            name=${this.label}
            id="split-by"
            label="Split"
            @change=${this.splitOnValue}
            .checked=${this.split}
            ?disabled=${this.disabled || this.splitDisabled}
            ?hidden=${!this.showSplit}>
          </checkbox-sk>
          <checkbox-sk
            title="Select all values without periods in the name."
            name=${this.label}
            id="select-primary"
            label="Primary"
            @change=${this.selectPrimary}
            .checked=${this._arePrimarySelected}
            ?disabled=${this.disabled}
            ?hidden=${!this.showPrimary}>
          </checkbox-sk>
          <checkbox-sk
            title="Select All"
            name=${this.label}
            id="select-all"
            label="All"
            @change=${this.selectAll}
            .checked=${this._isAllSelected}
            ?disabled=${this.disabled}
            ?hidden=${!this.showSelectAll}>
          </checkbox-sk>
        </div>
        <vaadin-multi-select-combo-box
          auto-expand-vertically
          label=${this.label}
          .items=${this.options}
          .selectedItems=${this.selectedItems}
          @selected-items-changed=${this.onValueChanged}
          ?readonly=${this.disabled}
          selected-items-on-top
          style="width: ${this._overlayWidth}; --vaadin-multi-select-combo-box-overlay-width: ${this
            ._overlayWidth}">
        </vaadin-multi-select-combo-box>
      </div>
    `;
  }

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

    this._checkboxSelected = false;

    // Update internal state AFTER dispatching so parent components
    // can read the old value synchronously during the event stack.
    this.selectedItems = selectedItems;
  }

  /**
   * Handles the change event for the "Split By" checkbox.
   * It updates the \`_splitBy\` property and dispatches a custom event
   * to notify that the split option has changed.
   *
   * @param e - The event triggered by the checkbox change.
   */
  private splitOnValue(e: Event) {
    this.split = (e.currentTarget as HTMLInputElement).checked;
    this.dispatchEvent(
      new CustomEvent('split-by-changed', {
        detail: {
          param: this.label,
          split: this.split,
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
      // Note: Updating this.selectedItems triggers render and updates combobox.
      // We manually dispatch value-changed to ensure the parent is notified.
      this.dispatchValueChanged();
    }
  }

  private dispatchValueChanged() {
    this.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: {
          value: this.selectedItems,
          checkboxSelected: this._checkboxSelected,
        },
        bubbles: true,
        composed: true,
      })
    );
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
      this.dispatchValueChanged();
    }
  }

  /**
   * Sets focus on the internal vaadin-multi-select-combo-box element.
   */
  focus() {
    this._comboBox?.focus();
  }

  /**
   * Opens the overlay (dropdown) of the internal
   * vaadin-multi-select-combo-box element.
   */
  openOverlay() {
    // Vaadin combo box doesn't always have 'click' to open?
    // Using internal dispatch or just reliance on `opened` property if available?
    // Original code used `.click()`.
    this._comboBox?.click();
  }

  /**
   * Disables the picker field and associated checkboxes.
   */
  disable() {
    this.disabled = true;
  }

  /**
   * Enables the picker field and associated checkboxes.
   */
  enable() {
    this.disabled = false;
    if (this._comboBox) {
      this._comboBox.removeAttribute('opened');
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
    if (this._comboBox) {
      this._comboBox.removeAttribute('value');
      this._comboBox.setAttribute('value', val);
    }
  }

  /**
   * Enables the split checkbox.
   */
  enableSplit() {
    this.splitDisabled = false;
  }

  /**
   * Disables the split checkbox and sets the split property to false.
   */
  disableSplit() {
    this.split = false;
    this.splitDisabled = true;
  }

  /**
   * Resets the selected items of the combo box.
   */
  reset() {
    this.selectedItems = [];
  }

  /**
   * Returns true if all options are currently selected.
   * @returns True if all options are selected, false otherwise.
   */
  private get _isAllSelected(): boolean {
    if (this.options.length === 0) {
      return false;
    }
    return this.selectedItems.length === this.options.length;
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
    // Check if lengths match (meaning no extra non-primary items selected? or just all primaries selected?)
    // Original logic:
    // return this._primaryOptions.length === this.selectedItems.length && show;
    // This allows exact match only.
    return this._primaryOptions.length === this.selectedItems.length && show;
  }

  /**

   * Returns true if the "Select All" checkbox should be shown.
   */
  get showSelectAll(): boolean {
    return this.options.length > 2 && this.index > 0;
  }

  /**
   * Returns true if the "Split" checkbox should be shown.
   */
  get showSplit(): boolean {
    return this.selectedItems.length > 1 && this.index > 0;
  }

  /**
   * Returns true if the "Primary" checkbox should be shown.
   */
  get showPrimary(): boolean {
    return (
      this._primaryOptions.length > 0 &&
      this._primaryOptions.length !== this.options.length &&
      this.index > 0
    );
  }

  get primaryOptions(): string[] {
    return this._primaryOptions;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'picker-field-sk': PickerFieldSk;
  }
}
