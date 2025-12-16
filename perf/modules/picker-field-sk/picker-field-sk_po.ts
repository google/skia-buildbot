import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class PickerFieldSkPO extends PageObject {
  get comboBox(): PageObjectElement {
    return this.bySelector('vaadin-multi-select-combo-box');
  }

  get splitByCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk#split-by');
  }

  get selectPrimaryCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk#select-primary');
  }

  get selectAllCheckbox(): PageObjectElement {
    return this.bySelector('checkbox-sk#select-all');
  }

  async getLabel(): Promise<string> {
    const label = await this.comboBox.getAttribute('label');
    return label || '';
  }

  async getSelectedItems(): Promise<string[]> {
    return this.comboBox.applyFnToDOMNode((n) => (n as any).selectedItems);
  }

  async openOverlay(): Promise<void> {
    await this.comboBox.click();
  }

  async select(value: string): Promise<void> {
    await this.openOverlay();
    await this.comboBox.type(value);
    await this.comboBox.press('Enter');
  }

  async clear(): Promise<void> {
    await this.comboBox.applyFnToDOMNode((n) => ((n as any).selectedItems = []));
  }

  /**
   * Removes a selected option from the combo box.
   *
   * It searches for a chip with a matching title (or inner text) and clicks its remove button.
   *
   * @param label The label of the option to remove (e.g., "Android").
   */
  async removeSelectedOption(label: string): Promise<void> {
    const chips = this.comboBox.bySelectorAll('vaadin-multi-select-combo-box-chip');
    const chipToRemove = await chips.find(async (chip) => {
      // Check title attribute first (observed behavior).
      const title = await chip.getAttribute('title');
      if (title && title.trim() === label) {
        return true;
      }
      // Fallback to innerText just in case.
      const text = await chip.innerText;
      return text.trim() === label;
    });

    if (!chipToRemove) {
      throw new Error(`Could not find chip with label '${label}'`);
    }

    await chipToRemove.applyFnToDOMNode((el) => {
      const root = el.shadowRoot || el;

      // Try to find the remove button by part name (standard Vaadin).
      const removePart = root.querySelector('[part~="remove-button"]');
      if (removePart) {
        (removePart as HTMLElement).click();
        return;
      }

      throw new Error('Could not find remove button in chip');
    });
  }

  async isDisabled(): Promise<boolean> {
    return this.comboBox.hasAttribute('readonly');
  }
}
