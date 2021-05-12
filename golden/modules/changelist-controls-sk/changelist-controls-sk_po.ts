import { BySelector, BySelectorAll, PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObjectElement, PageObjectElementList } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the ChangelistControlsSk component. */
export class ChangelistControlsSkPO extends PageObject {
  @BySelector('.inputs select')
  private patchsetDropDown!: PageObjectElement;

  @BySelector('.inputs radio-sk.include-master')
  private includeMasterRadio!: PageObjectElement;

  @BySelector('.inputs radio-sk.exclude-master')
  private excludeMasterRadio!: PageObjectElement;

  @BySelectorAll('.tryjob-container .tryjob')
  private tryjobs!: PageObjectElementList;

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
