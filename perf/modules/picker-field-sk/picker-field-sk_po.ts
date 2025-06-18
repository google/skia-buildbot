import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/**
 * Page Object for the PickerFieldSk component.
 */
export class PickerFieldSkPO extends PageObject {
  private get vaadinComboBox(): PageObjectElement {
    return this.bySelector('vaadin-multi-select-combo-box');
  }

  async click() {
    await this.vaadinComboBox.click();
  }

  async getSelectedItems(): Promise<string | null> {
    return this.vaadinComboBox.value;
  }

  async setPatchset(value: string) {
    await this.vaadinComboBox.enterValue(value);
  }

  async getLabel(): Promise<string | null> {
    return this.vaadinComboBox!.getAttribute('label');
  }

  /**
   * Gets the array of selected items in the picker field.
   */
  get selectedItems(): string[] {
    const selectedItems = this.vaadinComboBox?.getAttribute('selectedItems');
    if (selectedItems && Array.isArray(selectedItems)) {
      return selectedItems as string[];
    }
    return [];
  }

  /**
   * Opens the overlay of the picker field.
   */
  async openOverlay(): Promise<void> {
    if (!this.vaadinComboBox) {
      throw new Error('Vaadin combo box not found');
    }
    this.vaadinComboBox.click();
  }
}
