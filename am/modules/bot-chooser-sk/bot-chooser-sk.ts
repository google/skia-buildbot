/**
 * @module bot-chooser-sk
 * @description <h2><code>bot-chooser-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method
 * returns a Promise which will resolve when the user clicks OK after selecting
 * a bot or reject when the user clicks Cancel.
 * </p>
 *
 */
import { define } from 'elements-sk/define';
import { html, render, TemplateResult } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import { Incident } from '../json';

export class BotChooserSk extends HTMLElement {
  private dialog: HTMLDialogElement | null = null;

  private resolve: ((value: string | undefined)=> void) | null = null;

  private bots_to_incidents: Record<string, Incident[]> = {};

  private selected: string = '';

  private static template = (ele: BotChooserSk) => html`<dialog>${ele.displayDialogContents()}</dialog>`;

  connectedCallback(): void {
    this._render();
    this.dialog = $$('dialog', this);
  }

  /**
   * Display the dialog.
   *
   * @param bots_to_incidents Map of bots to their incidents.
   * @param bots_to_ignore Which bots should be ignored.
   * @returns Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  public open(bots_to_incidents: Record<string, Incident[]>, bots_to_ignore: string[]): Promise<string | undefined> {
    this.bots_to_incidents = {};
    Object.keys(bots_to_incidents).forEach((bot) => {
      if (bots_to_ignore.includes(bot)) {
        return;
      }
      this.bots_to_incidents[bot] = bots_to_incidents[bot];
    });
    this._render();
    this.dialog!.showModal();

    const selectElem = $$('select', this) as HTMLSelectElement;
    if (selectElem) {
      selectElem.focus();
      if (selectElem.length > 0) {
        // Select the first option.
        this.selected = selectElem.options[0].value;
        selectElem.selectedIndex = 0;
      }
    }
    return new Promise((resolve) => {
      this.resolve = resolve;
    });
  }

  private displayBotOptions(): TemplateResult[] {
    const botsHTML: TemplateResult[] = [];
    Object.keys(this.bots_to_incidents).forEach((bot) => {
      botsHTML.push(html`
        <option value=${bot}>${bot} [${this.bots_to_incidents[bot].map((i) => i.params.alertname).join(',')}]</option>
      `);
    });
    return botsHTML;
  }

  private displayDialogContents(): TemplateResult {
    if (Object.keys(this.bots_to_incidents).length === 0) {
      return html`
        <h2>No active bot alerts found</h2>
        <br/>
        <div class=buttons>
          <button @click=${this.dismiss}>OK</button>
        </div>
      `;
    }
    return html`
        <h2>Bots with active alerts</h2>
        <select size=10 @input=${this.input} @keyup=${this.keyup}>
          ${this.displayBotOptions()}
        </select>
        <div class=buttons>
          <button @click=${this.dismiss}>Cancel</button>
          <button @click=${this.confirm}>OK</button>
        </div>
      `;
  }

  private input(e: Event): void {
    this.selected = (e.target as HTMLInputElement).value;
  }

  private dismiss(): void {
    this.dialog!.close();
    this.resolve!(undefined);
  }

  private confirm(): void {
    this.dialog!.close();
    this.resolve!(this.selected);
  }

  private keyup(e: KeyboardEvent): void {
    if (e.key === 'Enter') {
      this.confirm();
    }
  }

  private _render(): void {
    render(BotChooserSk.template(this), this, { eventContext: this });
  }
}

define('bot-chooser-sk', BotChooserSk);
