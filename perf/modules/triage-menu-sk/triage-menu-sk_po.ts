import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { NewBugDialogSkPO } from '../new-bug-dialog-sk/new-bug-dialog-sk_po';
import { ExistingBugDialogSkPO } from '../existing-bug-dialog-sk/existing-bug-dialog-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class TriageMenuSkPO extends PageObject {
  get newBugDialog(): NewBugDialogSkPO {
    return this.poBySelector('new-bug-dialog-sk', NewBugDialogSkPO);
  }

  get existingBugDialog(): ExistingBugDialogSkPO {
    return this.poBySelector('existing-bug-dialog-sk', ExistingBugDialogSkPO);
  }

  get newBugButton(): PageObjectElement {
    return this.bySelector('#new-bug');
  }

  get existingBugButton(): PageObjectElement {
    return this.bySelector('#existing-bug');
  }

  get ignoreButton(): PageObjectElement {
    return this.bySelector('#ignore');
  }

  get ignoreToast(): PageObjectElement {
    return this.bySelector('#ignore_toast');
  }

  get nudgeSelectedButton(): PageObjectElement {
    return this.bySelector('.buttons > button.selected');
  }
}
