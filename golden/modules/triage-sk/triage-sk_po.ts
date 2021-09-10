import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { Label } from '../rpc_types';

/** A page object for the TriageSk component. */
export class TriageSkPO extends PageObject {
  private get positiveBtn(): PageObjectElement {
    return this.bySelector('button.positive');
  }

  private get negativeBtn(): PageObjectElement {
    return this.bySelector('button.negative');
  }

  private get untriagedBtn(): PageObjectElement {
    return this.bySelector('button.untriaged');
  }

  async getLabel(): Promise<Label> {
    const labels: Label[] = ['positive', 'negative', 'untriaged'];
    for (const label of labels) {
      if (await this.isButtonSelected(label)) {
        return label;
      }
    }
    throw new Error('No label selected');
  }

  async isButtonSelected(label: Label) {
    return this.getButtonForLabel(label).hasClassName('selected');
  }

  async clickButton(label: Label) { await this.getButtonForLabel(label).click(); }

  private getButtonForLabel(label: Label) {
    switch (label) {
      case 'positive':
        return this.positiveBtn;
      case 'negative':
        return this.negativeBtn;
      case 'untriaged':
        return this.untriagedBtn;
      default:
        throw new Error(`Unknown label: ${label}`);
    }
  }
}
