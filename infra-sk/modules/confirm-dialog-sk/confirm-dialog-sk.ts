/**
 * @module infra-sk/modules/confirm-dialog-sk
 * @description <h2><code>confirm-dialog-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
 * which will resolve when the user clicks OK or reject when the user clicks Cancel.
 * </p>
 *
 * @example
 *
 * <confirm-dialog-sk id="confirm_dialog"></confirm-dialog-sk>
 *
 * <script>
 *   (function(){
 *     $$('#confirm-dialog').open("Proceed with taking over the world?").then(() => {
 *       // Do some thing on confirm.
 *     }).catch(() => {
 *       // Do some thing on cancel.
 *     });
 *   })();
 * </script>
 *
 */
import { define } from 'elements-sk/define';
import dialogPolyfill from 'dialog-polyfill';
import { html, render } from 'lit-html';

import 'elements-sk/styles/buttons';

export class ConfirmDialogSk extends HTMLElement {
  private static template = (ele: ConfirmDialogSk) => html`
    <dialog @cancel=${ele.dismiss}>
      <h2>Confirm</h2>
      <div class=message>${ele.message}</div>
      <div class=buttons>
      <button class=dismiss @click=${ele.dismiss}>Cancel</button>
      <button class=confirm @click=${ele.confirm}>OK</button>
      </div>
    </dialog>
  `;

  private message = '';

  private dialog: HTMLDialogElement | null = null;

  private resolve = () => {};

  private reject = () => {};

  connectedCallback() {
    this.render();
    this.dialog = this.querySelector('dialog');
    dialogPolyfill.registerDialog(this.dialog!);
  }

  /**
   * Display the dialog.
   *
   * @param message Message to display. Text only, any markup will be escaped.
   * @returns Returns a Promise that resolves on OK, and rejects on Cancel.
   */
  open(message: string) {
    this.message = message;
    this.render();
    this.dialog!.showModal();
    return new Promise((resolve, reject) => {
      this.resolve = () => resolve(undefined);
      this.reject = reject;
    });
  }

  private dismiss() {
    this.dialog!.close();
    this.reject!();
  }

  private confirm() {
    this.dialog!.close();
    this.resolve!();
  }

  private render() {
    render(ConfirmDialogSk.template(this), this, { eventContext: this });
  }
}

define('confirm-dialog-sk', ConfirmDialogSk);
