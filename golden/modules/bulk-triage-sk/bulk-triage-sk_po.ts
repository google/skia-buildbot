import {BySelector, PageObject} from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import {PageObjectElement} from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the BulkTriageSkPO component. */
export class BulkTriageSkPO extends PageObject {
  @BySelector('p.cl')
  private cl!: Promise<PageObjectElement>;

  @BySelector('button.positive')
  private positiveBtn!: Promise<PageObjectElement>;

  @BySelector('button.negative')
  private negativeBtn!: Promise<PageObjectElement>;

  @BySelector('button.untriaged')
  private untriagedBtn!: Promise<PageObjectElement>;

  @BySelector('button.closest')
  private closestBtn!: Promise<PageObjectElement>;

  @BySelector('checkbox-sk.triage_all')
  private triageAllCheckBox!: Promise<PageObjectElement>;

  @BySelector('button.triage')
  private triageBtn!: Promise<PageObjectElement>;

  @BySelector('button.cancel')
  private cancelBtn!: Promise<PageObjectElement>;

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
