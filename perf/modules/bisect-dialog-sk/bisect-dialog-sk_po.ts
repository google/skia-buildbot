import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class BisectDialogSkPO extends PageObject {
  get dialog(): PageObjectElement {
    return this.bySelector('#bisect-dialog');
  }

  get closeIcon(): PageObjectElement {
    return this.bySelector('#bisectCloseIcon');
  }

  get testPathInput(): PageObjectElement {
    return this.bySelector('#testpath');
  }

  get bugIdInput(): PageObjectElement {
    return this.bySelector('#bug-id');
  }

  get startCommitInput(): PageObjectElement {
    return this.bySelector('#start-commit');
  }

  get endCommitInput(): PageObjectElement {
    return this.bySelector('#end-commit');
  }

  get storyInput(): PageObjectElement {
    return this.bySelector('#story');
  }

  get patchInput(): PageObjectElement {
    return this.bySelector('#patch');
  }

  get bisectButton(): PageObjectElement {
    return this.bySelector('#submit-button');
  }

  get spinner(): PageObjectElement {
    return this.bySelector('#dialog-spinner');
  }

  get bisectJobUrl(): PageObjectElement {
    return this.bySelector('#pinpoint-job-url');
  }

  async isDialogOpen(): Promise<boolean> {
    return await this.dialog.applyFnToDOMNode((el) => (el as HTMLDialogElement).open);
  }

  async getTestPath(): Promise<string> {
    return await this.testPathInput.value;
  }

  async setTestPath(value: string): Promise<void> {
    await this.testPathInput.enterValue(value);
  }

  async getBugId(): Promise<string> {
    return await this.bugIdInput.value;
  }

  async setBugId(value: string): Promise<void> {
    await this.bugIdInput.enterValue(value);
  }

  async getStartCommit(): Promise<string> {
    return await this.startCommitInput.value;
  }

  async setStartCommit(value: string): Promise<void> {
    await this.startCommitInput.enterValue(value);
  }

  async getEndCommit(): Promise<string> {
    return await this.endCommitInput.value;
  }

  async setEndCommit(value: string): Promise<void> {
    await this.endCommitInput.enterValue(value);
  }

  async getStory(): Promise<string> {
    return await this.storyInput.value;
  }

  async setStory(value: string): Promise<void> {
    await this.storyInput.enterValue(value);
  }

  async getPatch(): Promise<string> {
    return await this.patchInput.value;
  }

  async setPatch(value: string): Promise<void> {
    await this.patchInput.enterValue(value);
  }

  async clickBisectBtn(): Promise<void> {
    await this.bisectButton.click();
  }
}
