import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

/** A page object for the BulkTriageSkPO component. */
export class BulkTriageSkPO extends PageObject {
  async isAffectedChangelistIdVisible() {
    return (await this.selectOnePOE('p.cl')) !== null;
  }

  async getAffectedChangelistId() {
    return this.selectOnePOEThenApplyFn('p.cl', (el) => el.innerText);
  }

  isUntriagedBtnSelected() {
    return this.selectOnePOEThenApplyFn(
      'button.untriaged', async (el) => (await el.className).includes('selected'));
  }

  async clickUntriagedBtn() {
    await this.selectOnePOEThenApplyFn('button.untriaged', (el) => el.click());
  }

  isPositiveBtnSelected() {
    return this.selectOnePOEThenApplyFn(
      'button.positive', async (el) => (await el.className).includes('selected'));
  }

  async clickPositiveBtn() {
    await this.selectOnePOEThenApplyFn('button.positive', (el) => el.click());
  }

  isNegativeBtnSelected() {
    return this.selectOnePOEThenApplyFn(
      'button.negative', async (el) => (await el.className).includes('selected'));
  }

  async clickNegativeBtn() {
    await this.selectOnePOEThenApplyFn('button.negative', (el) => el.click());
  }

  isClosestBtnSelected() {
    return this.selectOnePOEThenApplyFn(
      'button.closest', async (el) => (await el.className).includes('selected'));
  }

  async clickClosestBtn() {
    await this.selectOnePOEThenApplyFn('button.closest', (el) => el.click());
  }

  async getTriageAllCheckboxLabel() {
    return this.selectOnePOEThenApplyFn('checkbox-sk.triage_all', (el) => el.innerText);
  }

  isTriageAllCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      'checkbox-sk.triage_all', (c) => (c as CheckOrRadio).checked);
  }

  async clickTriageAllCheckbox() {
    await this.selectOnePOEThenApplyFn('checkbox-sk.triage_all', (el) => el.click());
  }

  async getTriageBtnLabel() {
    return this.selectOnePOEThenApplyFn('button.triage', (el) => el.innerText);
  }

  async clickTriageBtn() {
    await this.selectOnePOEThenApplyFn('button.triage', (el) => el.click());
  }

  async clickCancelBtn() {
    await this.selectOnePOEThenApplyFn('button.cancel', (el) => el.click());
  }
};
