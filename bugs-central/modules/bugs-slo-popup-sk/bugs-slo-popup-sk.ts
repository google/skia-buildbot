/**
 * @module bugs-slo-popup-sk
 * @description <h2><code>bugs-slo-popup-sk</code></h2>
 *
 * A dialog that displays all bugs that are outside SLO.
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';

// TODO(rmistry): Generate this using go2ts.
export interface Issue {
  id: string,
  priority: string,
  link: string,

  slo_violation_reason: string,
}

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

  displayIssues(): TemplateResult[] {
    const priorities = Object.keys(this.priToSLOIssues);
    priorities.sort();
    const prioritiesHTML = [];
    for (let i = 0; i < priorities.length; i++) {
      const issues = this.priToSLOIssues[priorities[i]];
      const issuesHTML = [];
      for (let j = 0; j < issues.length; j++) {
        const issue = issues[j];
        issuesHTML.push(html`
          <tr>
            <td>
              <a href=${issue.link} rel=noopener target=_blank>${issue.id}</a>
            </td>
            <td>
              ${issue.slo_violation_reason}
            </td>
          </tr>
        `);
      }
      prioritiesHTML.push(html`
        <span class=priority-name>Priority: ${priorities[i]}</span>
        <table class=slo-table>
          ${issuesHTML}
        </table>
      `);
    }
    return prioritiesHTML;
  }

  open(mapOfIssues: Record<string, Issue[]>): void {
    this.priToSLOIssues = mapOfIssues;
    this._render();
    this.dialog!.showModal();
  }

  private closeClicked() {
    this.dialog!.close();
  }
}

define('bugs-slo-popup-sk', BugsSLOPopupSk);
