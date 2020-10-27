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
import { html } from 'lit-html';

import dialogPolyfill from 'dialog-polyfill';
import { $$ } from 'common-sk/modules/dom';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';

export interface Issue {
  id: string,
  priority: string,
  link: string,
}

export class BugsSLOPopupSk extends ElementSk {
private priToSLOIssues: Record<string, Issue[]> = {};

private dialog: HTMLDialogElement | null = null;


constructor() {
  super(BugsSLOPopupSk.template);
}

  private static template = (el: BugsSLOPopupSk) => html`
    <dialog class=slo-dialog>
      <span class=label>Priority</span>
      ${el.priToSLOIssues}
    </dialog>
    <button class=Done @click=${el.doneClicked}>Cancel</button>
    `;


  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
    console.log('HERE HERE');
    console.log(this.dialog);
    console.log(dialogPolyfill);
    dialogPolyfill.registerDialog(this.dialog!);
  }

  static get observedAttributes(): string[] {
    return ['client', 'source', 'query'];
  }

  open(mapOfIssues: Record<string, Issue[]>): void {
    this.priToSLOIssues = mapOfIssues;
    this._render();
    this.dialog!.showModal();
  }

  attributeChangedCallback(_name: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
    }
  }

  private doneClicked() {
    this.dialog!.close();
  }
}

define('bugs-slo-popup-sk', BugsSLOPopupSk);
