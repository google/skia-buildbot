import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { Label } from '../rpc_types';
import { LabelOrEmpty } from './triage-sk';

/** A page object for the TriageSk component. */
export class TriageSkPO extends PageObject {
  private get positiveBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('button.positive');
  }

  private get negativeBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('button.negative');
  }

  private get untriagedBtn(): Promise<PageObjectElement> {
    return this.selectOnePOE('button.untriaged');
  }

  async getLabelOrEmpty(): Promise<LabelOrEmpty> {
    const labels: Label[] = ['positive', 'negative', 'untriaged'];
    for (const label of labels) {
      if (await this.isButtonSelected(label)) {
        return label;
      }
    }
    return '';
  }

  async isButtonSelected(label: Label) {
    return (await this.getBtn(label)).hasClassName('selected');
  }

  async clickButton(label: Label) { await (await this.getBtn(label)).click(); }

  private getBtn(label: Label) {
    switch(label) {
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
