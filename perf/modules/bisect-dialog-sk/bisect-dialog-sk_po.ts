import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the BisectDialogSk component. */
export class BisectDialogSkPO extends PageObject {
  private get dialog(): PageObjectElement {
    return this.bySelector('dialog#bisect-dialog');
  }

  private get toastDialog(): PageObjectElement {
    return this.bySelector('toast-sk#bisect_toast');
  }

  private get testPathInput(): PageObjectElement {
    return this.bySelector('input#testpath');
  }

  private get bugIdInput(): PageObjectElement {
    return this.bySelector('input#bug-id');
  }

  private get startCommitInput(): PageObjectElement {
    return this.bySelector('input#start-commit');
  }

  private get endCommitInput(): PageObjectElement {
    return this.bySelector('input#end-commit');
  }

  private get storyInput(): PageObjectElement {
    return this.bySelector('input#story');
  }

  private get patchInput(): PageObjectElement {
    return this.bySelector('input#patch');
  }

  private get bisectBtn(): PageObjectElement {
    return this.bySelector('button#submit-button');
  }

  private get closeBtn(): PageObjectElement {
    return this.bySelector('button#close-btn');
  }

  async isDialogOpen(): Promise<boolean> {
    return this.dialog.applyFnToDOMNode((d) => (d as HTMLDialogElement).open);
  }

  async isToastDialogOpen(): Promise<boolean> {
    return this.toastDialog.hasAttribute('shown');
  }

  async getTestPath(): Promise<string> {
    return this.testPathInput.value;
  }

  async setTestPath(value: string): Promise<void> {
    await this.testPathInput.enterValue(value);
  }

  async getBugId(): Promise<string> {
    return this.bugIdInput.value;
  }

  async setBugId(value: string): Promise<void> {
    await this.bugIdInput.enterValue(value);
  }

  async getStartCommit(): Promise<string> {
    return this.startCommitInput.value;
  }

  async setStartCommit(value: string): Promise<void> {
    await this.startCommitInput.enterValue(value);
  }

  async getEndCommit(): Promise<string> {
    return this.endCommitInput.value;
  }

  async setEndCommit(value: string): Promise<void> {
    await this.endCommitInput.enterValue(value);
  }

  async getStory(): Promise<string> {
    return this.storyInput.value;
  }

  async setStory(value: string): Promise<void> {
    await this.storyInput.enterValue(value);
  }

  async getPatch(): Promise<string> {
    return this.patchInput.value;
  }

  async setPatch(value: string): Promise<void> {
    await this.patchInput.enterValue(value);
  }

  async clickBisectBtn(): Promise<void> {
    await this.bisectBtn.click();
  }

  async clickCloseBtn(): Promise<void> {
    await this.closeBtn.click();
  }
}
