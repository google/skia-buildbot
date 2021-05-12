import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the BulkTriageSkPO component. */
export class BulkTriageSkPO extends PageObject {
  private get cl(): Promise<PageObjectElement> {
    return this.bySelector('p.cl');
  }

  private get positiveBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.positive');
  }

  private get negativeBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.negative');
  }

  private get untriagedBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.untriaged');
  }

  private get closestBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.closest');
  }

  private get triageAllCheckBox(): Promise<PageObjectElement> {
    return this.bySelector('checkbox-sk.triage_all');
  }

  private get triageBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.triage');
  }

  private get cancelBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.cancel');
  }

  async isAffectedChangelistIdVisible() { return !(await this.cl).isEmpty(); }

  async getAffectedChangelistId() { return (await this.cl).innerText; }

  async isUntriagedBtnSelected() { return (await this.untriagedBtn).hasClassName('selected'); }

  async clickUntriagedBtn() { await (await this.untriagedBtn).click(); }

  async isPositiveBtnSelected() { return (await this.positiveBtn).hasClassName('selected'); }

  async clickPositiveBtn() { return (await this.positiveBtn).click(); }

  async isNegativeBtnSelected() { return (await this.negativeBtn).hasClassName('selected'); }

  async clickNegativeBtn() { await (await this.negativeBtn).click(); }

  async isClosestBtnSelected() { return (await this.closestBtn).hasClassName('selected'); }

  async clickClosestBtn() { await (await this.closestBtn).click(); }

  async getTriageAllCheckboxLabel() {  return (await this.triageAllCheckBox).innerText; }

  async isTriageAllCheckboxChecked() {
    return (await this.triageAllCheckBox).applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickTriageAllCheckbox() { await (await this.triageAllCheckBox).click(); }

  async getTriageBtnLabel() { return (await this.triageBtn).innerText; }

  async clickTriageBtn() { await (await this.triageBtn).click(); }

  async clickCancelBtn() { await (await this.cancelBtn).click(); }
}
