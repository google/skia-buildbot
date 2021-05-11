import { BySelector, BySelectorAll, PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { asyncMap } from '../../../infra-sk/modules/async';

/** A page object for the ChangelistControlsSk component. */
export class ChangelistControlsSkPO extends PageObject {
  @BySelector('.inputs select')
  private patchsetDropDown!: Promise<PageObjectElement>;

  @BySelector('.inputs radio-sk.include-master')
  private includeMasterRadio!: Promise<PageObjectElement>;

  @BySelector('.inputs radio-sk.exclude-master')
  private excludeMasterRadio!: Promise<PageObjectElement>;

  @BySelectorAll('.tryjob-container .tryjob')
  private tryjobs!: Promise<PageObjectElement[]>;

  isVisible() {
    return this.element.applyFnToDOMNode((el) => el.children.length > 0);
  }

  async getPatchset() { return (await this.patchsetDropDown).value; }

  async setPatchset(value: string) { await (await this.patchsetDropDown).enterValue(value); }

  async isExcludeResultsFromPrimaryRadioChecked() {
    return (await this.excludeMasterRadio).applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickExcludeResultsFromPrimaryRadio() { await (await this.excludeMasterRadio).click(); }

  async isShowAllResultsRadioChecked() {
    return (await this.includeMasterRadio).applyFnToDOMNode((el) => (el as CheckOrRadio).checked);
  }

  async clickShowAllResultsRadio() { await (await this.includeMasterRadio).click(); }

  async getTryJobs() { return asyncMap(this.tryjobs, (tryjob) => tryjob.innerText); }
}
