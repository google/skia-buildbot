import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the ExistingBugDialogSk component. */
export class ExistingBugDialogSkPO extends PageObject {
  private get dialog(): PageObjectElement {
    return this.bySelector('dialog');
  }

  private get bugIdInput(): PageObjectElement {
    return this.bySelector('input#bug_id');
  }

  private get submitBtn(): PageObjectElement {
    return this.bySelector('button.submit');
  }

  private get closeBtn(): PageObjectElement {
    return this.bySelector('button.close');
  }

  async isDialogOpen() {
    return this.dialog.applyFnToDOMNode((d) => (d as HTMLDialogElement).open);
  }

  async setBugId(bugId: string): Promise<void> {
    await this.bugIdInput.click();
    await this.bugIdInput.enterValue(bugId);
  }

  async getBugId(): Promise<string> {
    return this.bugIdInput.value;
  }

  async clickSubmitBtn() {
    await this.submitBtn.click();
  }

  async clickCloseBtn() {
    await this.closeBtn.click();
  }
}
