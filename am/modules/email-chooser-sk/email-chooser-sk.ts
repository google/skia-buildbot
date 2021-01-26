/**
 * @module email-chooser-sk
 * @description <h2><code>email-chooser-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
 * which will resolve when the user clicks OK after selecting
 * an email or reject when the user clicks Cancel.
 * </p>
 *
 */
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { html, render, TemplateResult } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';

export class EmailChooserSk extends HTMLElement {
  private dialog: HTMLDialogElement | null = null;

  private resolve: ((value: string | PromiseLike<string>)=> void) | null = null;

  private reject: ((reason?: any)=> void)|null = null;

  private emails: string[] = [];

  private owner: string = '';

  private selected: string = '';

  private static template = (ele: EmailChooserSk) => html`<dialog>
  <h2>Assign</h2>
  <select size=10 @input=${ele._input}>
    <option value='' selected>(un-assign)</option>
    ${ele.emails.map((email: string) => ele.displayEmail(email))}
  </select>
  <div class=buttons>
    <button @click=${ele._dismiss}>Cancel</button>
    <button @click=${ele._confirm}>OK</button>
  </div>
</dialog>`;

  connectedCallback(): void {
    this._render();
    this.dialog = $$('dialog', this);
    dialogPolyfill.registerDialog(this.dialog!);
  }

  displayEmail(email: string): TemplateResult {
    if (this.owner === email) {
      return html`<option value=${email}>${email} (alert owner)</option>`;
    }
    return html`<option value=${email}>${email}</option>`;
  }

  /**
   * Display the dialog.
   *
   * @param emails {Array} List of emails to choose from.
   * @param owner {String} The owner of this incident if available. Optional.
   * @returns {Promise} Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  open(emails: string[], owner: string): Promise<string> {
    this.emails = emails;
    this.owner = owner;
    this._render();
    this.dialog!.showModal();
    ($$('select', this) as HTMLSelectElement).focus();
    return new Promise((resolve, reject) => {
      this.resolve = resolve;
      this.reject = reject;
    });
  }

  _input(e: Event): void {
    this.selected = (e.target as HTMLInputElement).value;
  }

  _dismiss(): void {
    this.dialog!.close();
    this.reject!();
  }

  _confirm(): void {
    this.dialog!.close();
    this.resolve!(this.selected);
  }

  _render(): void {
    render(EmailChooserSk.template(this), this, { eventContext: this });
  }
}

define('email-chooser-sk', EmailChooserSk);
