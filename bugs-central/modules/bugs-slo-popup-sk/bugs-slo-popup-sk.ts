/**
 * @module bugs-slo-popup-sk
 * @description <h2><code>bugs-slo-popup-sk</code></h2>
 *
 * A dialog that displays all bugs that are outside SLO.
 *
 * @attr chart_type {string} The type of the chart. Eg: open/slo.
 *
 * @attr chart_title {string} The title of the chart.
 *
 * @attr data {string} Data for the chart. Eg: '[["Month", "Days"], ["Jan", 31], ["Feb", 28], ["Mar", 31]]'.
 *
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';

import { $$ } from 'common-sk/modules/dom';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';

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

  static get observedAttributes(): string[] {
    return ['client', 'source', 'query'];
  }

  displayIssues(): TemplateResult[] {
    // return html`
    // HERE HERE HERE`;
    const priorities = Object.keys(this.priToSLOIssues);
    priorities.sort();
    const prioritiesHTML = [];
    for (let i = 0; i < priorities.length; i++) {
      // const priorityHTML = html`<b>Priority: ${priorities[i]}</b>`;
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
        // issuesHTML.push(html``);
      }
      prioritiesHTML.push(html`
        <span class=priority-name>Priority: ${priorities[i]}</span>
        <table class=slo-table>
          ${issuesHTML}
        </table>
      `);
      console.log(priorities[i]);
      // prioritiesHTML.push(html`<b>Priority: ${priorities[i]}</b><br/>`); // CSS HERE?
    }
    // console.log(priorities);
    // priorities.sort();
    // console.log(this.priToSLOIssues);
    // console.log(priorities);
    console.log('HERE HERE');
    console.log(prioritiesHTML);
    return prioritiesHTML;
  }

  open(mapOfIssues: Record<string, Issue[]>): void {
    this.priToSLOIssues = mapOfIssues;
    console.log('IN OPEN WITH');
    console.log(mapOfIssues);
    this._render();
    this.dialog!.showModal();
  }

  attributeChangedCallback(_name: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
    }
  }

  private closeClicked() {
    this.dialog!.close();
  }
}

define('bugs-slo-popup-sk', BugsSLOPopupSk);
