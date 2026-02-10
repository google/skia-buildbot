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

  private get okBtn(): PageObjectElement {
    return this.bySelector('button.ok');
  }

  private get cancelBtn(): PageObjectElement {
    return this.bySelector('button.cancel');
  }

  async isDialogOpen() {
    return await this.dialog.applyFnToDOMNode((d) => (d as HTMLDialogElement).open);
  }

  async setBugId(bugId: string): Promise<void> {
    await this.bugIdInput.click();
    await this.bugIdInput.enterValue(bugId);
  }

  async getBugId(): Promise<string> {
    return await this.bugIdInput.value;
  }

  async clickOkBtn() {
    await this.okBtn.click();
  }

  async clickCancelBtn() {
    await this.cancelBtn.click();
  }
}
