/**
 * @module modules/device-editor-sk
 * @description <h2><code>device-editor-sk</code></h2>
 *
 * Displays a dialog to clear the device dimensions or edit them.
 *
 * It emits events when the user takes actions.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';

// ClearDeviceEvent is emitted when the user wishes to clear the device.
export const ClearDeviceEvent = 'clear_device';

export class DeviceEditorSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private machineID: string = '';

  private static template = (ele: DeviceEditorSk) => html`
  <dialog>
      <h1>Edit device dimensions for ${ele.machineID}</h1>
      <button title="Make the machine forget it had any device attached."
          @click=${ele.clearClick} class=clear>Clear Device Dimensions</button>
      <button title="Do nothing except close the dialog box. The machine remains unchanged."
          @click=${ele.cancelClick} class=cancel>Cancel</button>
    </div>
  </dialog>
  `;

  constructor() {
    super(DeviceEditorSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector<HTMLDialogElement>('dialog');
    dialogPolyfill.registerDialog(this.dialog!);
  }

  show(id: string): void {
    this.machineID = id;
    this._render();
    this.dialog?.show();
  }

  private cancelClick() {
    this.dialog?.close();
  }

  private clearClick() {
    this.dialog?.close();
    this.dispatchEvent(new CustomEvent(ClearDeviceEvent, {
      bubbles: true,
      detail: this.machineID,
    }));
  }
}

define('device-editor-sk', DeviceEditorSk);
