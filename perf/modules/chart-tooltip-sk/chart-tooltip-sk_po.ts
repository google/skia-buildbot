import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { CommitRangeSkPO } from '../commit-range-sk/commit-range-sk_po';
import { TriageMenuSkPO } from '../triage-menu-sk/triage-menu-sk_po';

export class ChartTooltipSkPO extends PageObject {
  get container(): PageObjectElement {
    return this.bySelector('.container');
  }

  get closeButton(): PageObjectElement {
    return this.bySelector('#closeIcon');
  }

  get title(): PageObjectElement {
    return this.bySelector('h3');
  }

  get table(): PageObjectElement {
    return this.bySelector('.table');
  }

  get commitRangeLink(): CommitRangeSkPO {
    return this.poBySelector('commit-range-sk', CommitRangeSkPO);
  }

  get getTriageMenu(): TriageMenuSkPO {
    return this.poBySelector('triage-menu-sk', TriageMenuSkPO);
  }

  get pointLinks(): PageObjectElement {
    return this.bySelector('#tooltip-point-links');
  }

  get triageMenu(): PageObjectElement {
    return this.bySelector('#triage-menu');
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

  get newBugDialog(): PageObjectElement {
    return this.bySelector('new-bug-dialog-sk');
  }

  get existingBugDialog(): PageObjectElement {
    return this.bySelector('existing-bug-dialog-sk');
  }

  get ignoreToast(): PageObjectElement {
    return this.bySelector('#ignore_toast');
  }

  get bisectButton(): PageObjectElement {
    return this.bySelector('#bisect');
  }

  get tryJobButton(): PageObjectElement {
    return this.bySelector('#try-job');
  }

  get userIssue(): PageObjectElement {
    return this.bySelector('#tooltip-user-issue-sk');
  }

  get jsonSourceDialog(): PageObjectElement {
    return this.bySelector('#json-source-dialog');
  }

  get tryJobDialog(): PageObjectElement {
    return this.bySelector('#pinpoint-try-job-dialog-sk');
  }

  get anomalyDetails(): PageObjectElement {
    return this.bySelector('#anomaly-details');
  }

  get unassociateBugButton(): PageObjectElement {
    return this.bySelector('#unassociate-bug-button');
  }

  async clickBisectDialogButton(): Promise<void> {
    await this.bisectButton.click();
  }
}
