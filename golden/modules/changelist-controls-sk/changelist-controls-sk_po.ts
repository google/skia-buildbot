import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

/** A page object for the ChangelistControlsSk component. */
export class ChangelistControlsSkPO extends PageObject {

  isVisible() {
    return this.element.applyFnToDOMNode((el) => el.children.length > 0);
  }

  getPatchSet() {
    return this.selectOnePOEThenApplyFn('.inputs select', (el) => el.value);
  }

  setPatchSet(value: string) {
    return this.selectOnePOEThenApplyFn('.inputs select', (el) => el.enterValue(value));
  }

  isExcludeResultsFromPrimaryRadioChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.inputs radio-sk.exclude-master', (el) => (el as CheckOrRadio).checked);
  }

  async clickExcludeResultsFromPrimaryRadio() {
    await this.selectOnePOEThenApplyFn('.inputs radio-sk.exclude-master', (el) => el.click());
  }

  isShowAllResultsRadioChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.inputs radio-sk.include-master', (el) => (el as CheckOrRadio).checked);
  }

  async clickShowAllResultsRadio() {
    await this.selectOnePOEThenApplyFn('.inputs radio-sk.include-master', (el) => el.click());
  }

  getTryJobs() {
    return this.selectAllPOEThenMap('.tryjob-container .tryjob', (el) => el.innerText);
  }

};
