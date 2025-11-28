import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';
import { PickerFieldSkPO } from '../picker-field-sk/picker-field-sk_po';

export class TestPickerSkPO extends PageObject {
  get pickerFields(): PageObjectElementList {
    return this.bySelectorAll('picker-field-sk');
  }

  get plotButton(): PageObjectElement {
    return this.bySelector('#plot-button');
  }

  get spinner(): PageObjectElement {
    return this.bySelector('spinner-sk');
  }

  async getPickerField(index: number): Promise<PickerFieldSkPO> {
    const fields = await this.pickerFields;
    return new PickerFieldSkPO(await fields.item(index));
  }

  async waitForPickerField(index: number): Promise<void> {
    const selector = `picker-field-sk:nth-of-type(${index + 1})`;
    await this.poll(
      async () => !(await this.bySelector(selector).isEmpty()),
      `Waiting for ${selector}`
    );
  }

  async waitForSpinnerInactive(): Promise<void> {
    await this.poll(
      async () => !(await this.bySelector('spinner-sk:not([active])').isEmpty()),
      'Waiting for spinner inactive'
    );
  }

  async clickPlotButton(): Promise<void> {
    // Wait for button to be enabled
    await this.poll(
      async () => !(await this.bySelector('#plot-button:not([disabled])').isEmpty()),
      'Waiting for plot button enabled'
    );
    await this.plotButton.click();
  }

  private async poll(
    checkFn: () => Promise<boolean>,
    message: string,
    timeout = 5000,
    interval = 100
  ): Promise<void> {
    const startTime = Date.now();
    while (Date.now() - startTime < timeout) {
      if (await checkFn()) {
        return;
      }
      await new Promise((resolve) => setTimeout(resolve, interval));
    }
    throw new Error(`Timeout: ${message}`);
  }
}
