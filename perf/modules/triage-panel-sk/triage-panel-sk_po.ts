import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';
import { ExistingBugDialogSkPO } from '../existing-bug-dialog-sk/existing-bug-dialog-sk_po';

export class TriagePanelSkPO extends PageObject {
  get collapseButton(): PageObjectElement {
    return this.bySelector('.collapse-btn');
  }

  get newBucketInput(): PageObjectElement {
    return this.bySelector('.add-bucket input');
  }

  get addBucketButton(): PageObjectElement {
    return this.bySelector('.add-bucket button[title="Add Bucket"]');
  }

  get copyAllButton(): PageObjectElement {
    return this.bySelector('.add-bucket button[title="Copy all panel buckets to clipboard"]');
  }

  get pasteAllButton(): PageObjectElement {
    return this.bySelector('.add-bucket button[title="Paste panel state from clipboard"]');
  }

  get bucketCards(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card');
  }

  get bucketTitles(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card h3');
  }

  get ignoreButtons(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card .ignore-btn');
  }

  get newBugButtons(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card .new-bug-btn');
  }

  get existingBugButtons(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card .existing-bug-btn');
  }

  get applyButtons(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card .apply-btn');
  }

  get textareas(): PageObjectElementList {
    return this.bySelectorAll('.bucket-card .bucket-textarea');
  }

  get existingBugDialog(): ExistingBugDialogSkPO {
    return this.poBySelector('existing-bug-dialog-sk', ExistingBugDialogSkPO);
  }
}
