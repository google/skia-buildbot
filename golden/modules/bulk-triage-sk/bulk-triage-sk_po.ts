import { BySelector, PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the BulkTriageSkPO component. */
export class BulkTriageSkPO extends PageObject {
  @BySelector('p.cl')
  private cl!: PageObjectElement;

  @BySelector('button.positive')
  private positiveBtn!: PageObjectElement;

  @BySelector('button.negative')
  private negativeBtn!: PageObjectElement;

  @BySelector('button.untriaged')
  private untriagedBtn!: PageObjectElement;

  @BySelector('button.closest')
  private closestBtn!: PageObjectElement;

  @BySelector('checkbox-sk.triage_all')
  private triageAllCheckBox!: PageObjectElement;

  @BySelector('button.triage')
  private triageBtn!: PageObjectElement;

  @BySelector('button.cancel')
  private cancelBtn!: PageObjectElement;

  async isAffectedChangelistIdVisible() { return !(await this.cl.isEmpty()); }

  async getAffectedChangelistId() { return this.cl.innerText; }

  async isUntriagedBtnSelected() { return this.untriagedBtn.hasClassName('selected'); }

  async clickUntriagedBtn() { await this.untriagedBtn.click(); }

  async isPositiveBtnSelected() { return this.positiveBtn.hasClassName('selected'); }

  async clickPositiveBtn() { await this.positiveBtn.click(); }

  async isNegativeBtnSelected() { return this.negativeBtn.hasClassName('selected'); }

  async clickNegativeBtn() { await this.negativeBtn.click(); }

  async isClosestBtnSelected() { return this.closestBtn.hasClassName('selected'); }

  async clickClosestBtn() { await this.closestBtn.click(); }

  async getTriageAllCheckboxLabel() {  return this.triageAllCheckBox.innerText; }

  async isTriageAllCheckboxChecked() {
    return this.triageAllCheckBox.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickTriageAllCheckbox() { await this.triageAllCheckBox.click(); }

  async getTriageBtnLabel() { return this.triageBtn.innerText; }

  async clickTriageBtn() { await this.triageBtn.click(); }

  async clickCancelBtn() { await this.cancelBtn.click(); }
}
