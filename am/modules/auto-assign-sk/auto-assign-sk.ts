/**
 * @module auto-assign-sk
 * @description <h2><code>auto-assign-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
 * which will resolve with selected incidents when the user clicks OK or resolve to undefined
 * when the user clicks Cancel. The promise will be rejected if there is an error condition.
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { $$ } from '../../../infra-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Incident } from '../json';

export class AutoAssignSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private resolve: ((value: string[] | undefined) => void) | null = null;

  private incidents: Incident[] = [];

  private selected: string[] = [];

  private static template = (ele: AutoAssignSk) =>
    html`<dialog>${ele.displayDialogContents()}</dialog>`;

  constructor() {
    super(AutoAssignSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
  }

  /**
   * Display the dialog.
   *
   * @param incidents List of incidents to choose from.
   * @returns Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  public open(incidents: Incident[]): Promise<string[] | undefined> {
    this.incidents = incidents;
    this._render();
    this.dialog!.showModal();
    const select = $$('select', this) as HTMLSelectElement;
    if (select) {
      select.focus();
      this.selected = [];
      Array.from(select.selectedOptions).forEach((option) => {
        this.selected.push(option.value);
      });
    }
    return new Promise((resolve) => {
      this.resolve = resolve;
    });
  }

  private displayDialogContents(): TemplateResult {
    if (Object.keys(this.incidents).length === 0) {
      return html`
        <h2>No unassigned active alerts found</h2>
        <br />
        <div class="buttons">
          <button @click=${this.dismiss}>OK</button>
        </div>
      `;
    }
    return html`
      <h2>The selected active alerts will be auto-assigned to owners</h2>
      <select size="10" @input=${this.input} @keyup=${this.keyup} multiple>
        ${this.incidents.map((incident: Incident) =>
          this.displayIncident(incident)
        )}
      </select>
      <div class="buttons">
        <button @click=${this.dismiss}>Cancel</button>
        <button @click=${this.confirm}>OK</button>
      </div>
    `;
  }

  private displayIncident(incident: Incident): TemplateResult {
    let display = incident.params.alertname;
    const abbr = incident.params.abbr;
    if (abbr) {
      display += ` ${abbr}`;
    }
    if (display.length > 33) {
      display = `${display.slice(0, 30)}...`;
    }
    display += ` -> ${incident.params.owner}`;
    return html`<option value=${incident.key} selected>${display}</option>`;
  }

  private input(e: Event): void {
    this.selected = [];
    Array.from((e.target as HTMLSelectElement).selectedOptions).forEach(
      (option) => {
        this.selected.push(option.value);
      }
    );
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
}

define('auto-assign-sk', AutoAssignSk);
