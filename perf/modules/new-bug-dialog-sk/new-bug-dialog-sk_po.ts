import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class NewBugDialogSkPO extends PageObject {
  get dialog(): PageObjectElement {
    return this.bySelector('#new-bug-dialog');
  }

  get closeIcon(): PageObjectElement {
    return this.bySelector('#closeIcon');
  }

  get titleInput(): PageObjectElement {
    return this.bySelector('#title');
  }

  get descriptionTextarea(): PageObjectElement {
    return this.bySelector('#description');
  }

  get assigneeInput(): PageObjectElement {
    return this.bySelector('#assignee');
  }

  get ccsInput(): PageObjectElement {
    return this.bySelector('#ccs');
  }

  get loadingPopup(): PageObjectElement {
    return this.bySelector('#loading-popup');
  }

  async getBugTitle(): Promise<string> {
    return this.titleInput.value;
  }

  async getDescription(): Promise<string> {
    return this.descriptionTextarea.value;
  }

  async getAssignee(): Promise<string> {
    return this.assigneeInput.value;
  }

  async getCcs(): Promise<string> {
    return this.ccsInput.value;
  }
}
