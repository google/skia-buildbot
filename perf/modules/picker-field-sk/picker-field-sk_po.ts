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

  async getSelectedItems(): Promise<string[]> {
    return this.comboBox.applyFnToDOMNode((n) => (n as any).selectedItems);
  }

  async openOverlay(): Promise<void> {
    await this.comboBox.click();
  }

  async search(value: string): Promise<void> {
    await this.openOverlay();
    await this.comboBox.type(value);
    await this.comboBox.press('Enter');
  }

  async clear(): Promise<void> {
    await this.comboBox.applyFnToDOMNode((n) => ((n as any).selectedItems = []));
  }

  async isDisabled(): Promise<boolean> {
    return this.comboBox.hasAttribute('readonly');
  }
}
