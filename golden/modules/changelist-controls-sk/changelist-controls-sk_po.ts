import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObjectElement, PageObjectElementList } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the ChangelistControlsSk component. */
export class ChangelistControlsSkPO extends PageObject {
  private get patchsetDropDown(): PageObjectElement {
    return this.bySelector('.inputs select');
  }

  private get includeMasterRadio(): PageObjectElement {
    return this.bySelector('.inputs radio-sk.include-master');
  }

  private get excludeMasterRadio(): PageObjectElement {
    return this.bySelector('.inputs radio-sk.exclude-master');
  }

  private get tryjobs(): PageObjectElementList {
    return this.bySelectorAll('.tryjob-container .tryjob');
  }

  isVisible() {
    return this.element.applyFnToDOMNode((el) => el.children.length > 0);
  }

  async getPatchset() { return this.patchsetDropDown.value; }

  async setPatchset(value: string) { await this.patchsetDropDown.enterValue(value); }

  async isExcludeResultsFromPrimaryRadioChecked() {
    return this.excludeMasterRadio.applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickExcludeResultsFromPrimaryRadio() { await this.excludeMasterRadio.click(); }

  async isShowAllResultsRadioChecked() {
    return this.includeMasterRadio.applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickShowAllResultsRadio() { await this.includeMasterRadio.click(); }

  async getTryJobs() { return this.tryjobs.map((tryjob) => tryjob.innerText); }
}
