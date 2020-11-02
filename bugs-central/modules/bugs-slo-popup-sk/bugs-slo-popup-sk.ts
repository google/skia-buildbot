/**
 * @module bugs-slo-popup-sk
 * @description <h2><code>bugs-slo-popup-sk</code></h2>
 *
 * A dialog that displays all bugs that are outside SLO.
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';

import { $$ } from 'common-sk/modules/dom';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';
import { Issue } from '../json';

export class BugsSLOPopupSk extends ElementSk {
  private priToSLOIssues: Record<string, Issue[]> = {};

  private dialog: HTMLDialogElement | null = null;


  constructor() {
    super(BugsSLOPopupSk.template);
  }

  private static template = (el: BugsSLOPopupSk) => html`
    <dialog class=slo-dialog>
      <button class=done @click=${el.closeClicked}>Close</button>
      ${el.displayIssues()}
    </dialog>
    `;


  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
  }

  // Opens up the dialog and displays the specified map of issues.
  open(mapOfIssues: Record<string, Issue[]>): void {
    this.priToSLOIssues = mapOfIssues;
    this._render();
    this.dialog!.showModal();
    dialogPolyfill.registerDialog(this.dialog!);
  }

  private displayIssues(): TemplateResult[] {
    const priorities = Object.keys(this.priToSLOIssues);
    priorities.sort();
    const prioritiesHTML: TemplateResult[] = [];
    priorities.forEach((priority) => {
      const issues = this.priToSLOIssues[priority];
      const issuesHTML: TemplateResult[] = [];
      issues.forEach((issue) => {
        issuesHTML.push(html`
          <tr>
            <td>
              <a href=${issue.link} target=_blank>${issue.id}</a>
            </td>
            <td>
              ${issue.slo_violation_reason}
            </td>
          </tr>
        `);
      });
      prioritiesHTML.push(html`
        <span class=priority-name>Priority: ${priority}</span>
        <table class=slo-table>
          ${issuesHTML}
        </table>
      `);
    });
    return prioritiesHTML;
  }

  private closeClicked() {
    this.dialog!.close();
  }
}

define('bugs-slo-popup-sk', BugsSLOPopupSk);
