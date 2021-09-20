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
import { $$ } from 'common-sk/modules/dom';
import dialogPolyfill from 'dialog-polyfill';
import 'elements-sk/styles/buttons';
import 'elements-sk/checkbox-sk';
import { SwarmingDimensions } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// ClearDeviceEvent is emitted when the user wishes to clear the device.
export const ClearDeviceEvent = 'clear_device';

// ClearDeviceEvent is emitted when the user wishes to clear the device.
export const UpdateDimensionsEvent = 'update_dimensions';

export interface UpdateDimensionsDetails {
  machineID: string;
  sshUserIP: string;
  specifiedDimensions: {
    gpu: string[];
    cpu: string[];
  };
}

export class DeviceEditorSk extends ElementSk {
  private infoDialog: HTMLDialogElement | null = null;

  private confirmDialog: HTMLDialogElement | null = null;

  private machineID: string = '';

  private sshUserIP: string = '';

  private dimensions: SwarmingDimensions = {};

  private static template = (ele: DeviceEditorSk) => html`
  <dialog class="info">
    <h1>Edit device dimensions for ${ele.machineID}</h1>

    <div class="center">Set the below boxes to indicate a ChromeOS machine</div>
    <table>
      <tr>
        <td>User and IP address:</td>
        <td><input placeholder="e.g. root@skia-pixelbook-01" type=text
                   id="user_ip" .value=${ele.sshUserIP}></td>
      </tr>
      <tr>
        <td>GPU (from chrome://gpu/). Comma seperated if multiple.</td>
        <td><input type=text id="chromeos_gpu"
                   .value=${ele.displayDimensions('gpu')}></td>
      </tr>
      <tr>
        <td>CPU (x86,x86_64,arm,arm32,arm64). Comma seperated if multiple.</td>
        <td><input type=text id="chromeos_cpu"
                   .value=${ele.displayDimensions('cpu')}></td>
      </tr>
    </table>

    <div class=buttons>
      <button title="Make the machine forget it had any device attached."
              @click=${ele.confirmClearClick} class=clear>Clear Device Dimensions</button>
      <button title="Apply the above dimensions and user ip"
              @click=${ele.applyUpdates} class=apply>Apply Updates</button>
      <button title="Do nothing except close the dialog box. The machine remains unchanged."
              @click=${ele.cancelClick} class=cancel>Cancel</button>
    </div>
  </dialog>
  <dialog class="confirm">
    <div class="warning">
      Clearing the dimensions, especially for ChromeOS can be annoying to undo.
      <br>
      Are you sure?
      <br>
    </div>

    <div>
      <button title="Do nothing except close the dialog box. The machine remains unchanged."
              @click=${ele.cancelClick} class=cancel>Cancel</button>
      <button title="Make the machine forget it had any device attached."
              @click=${ele.clearClick} class=clear_yes_im_sure>Yes, clear the dimensions</button>
    </div>
  </dialog>
  `;

  constructor() {
    super(DeviceEditorSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.infoDialog = this.querySelector<HTMLDialogElement>('dialog.info');
    this.confirmDialog = this.querySelector<HTMLDialogElement>('dialog.confirm');
    dialogPolyfill.registerDialog(this.infoDialog!);
    dialogPolyfill.registerDialog(this.confirmDialog!);
  }

  private applyUpdates(): void {
    const gpus = $$<HTMLInputElement>('input#chromeos_gpu', this)!.value.split(',');
    const cpus = $$<HTMLInputElement>('input#chromeos_cpu', this)!.value.split(',');

    this.dispatchEvent(new CustomEvent<UpdateDimensionsDetails>(UpdateDimensionsEvent, {
      bubbles: true,
      detail: {
        machineID: this.machineID,
        sshUserIP: $$<HTMLInputElement>('input#user_ip', this)!.value,
        specifiedDimensions: {
          gpu: gpus,
          cpu: cpus,
        },
      },
    }));
    this.infoDialog?.close();
  }

  private cancelClick(): void {
    this.infoDialog?.close();
    this.confirmDialog?.close();
  }

  private clearClick(): void {
    this.infoDialog?.close();
    this.confirmDialog?.close();
    this.dispatchEvent(new CustomEvent<string>(ClearDeviceEvent, {
      bubbles: true,
      detail: this.machineID,
    }));
  }

  private confirmClearClick(): void {
    this.infoDialog?.close();
    this.confirmDialog?.showModal();
  }

  private displayDimensions(key: string): string {
    // If sshUserIP isn't set, we don't want to give the wrong impression that we can
    // override the GPU/CPU for any device - we only set it for ChromeOS devices because
    // we can't easily interrogate them automatically.
    if (this.sshUserIP === '') {
      return '';
    }
    return this.dimensions![key]?.join(',') || '';
  }

  show(dims: SwarmingDimensions, sshUserIP: string): void {
    if (!dims || !dims.id) {
      return;
    }
    this.machineID = dims.id[0];
    this.dimensions = dims;
    this.sshUserIP = sshUserIP;
    this._render();
    this.infoDialog?.showModal();
  }
}

define('device-editor-sk', DeviceEditorSk);
