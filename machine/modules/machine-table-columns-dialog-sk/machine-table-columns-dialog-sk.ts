/**
 * @module modules/machine-table-columns-dialog-sk
 * @description <h2><code>machine-table-columns-dialog-sk</code></h2>
 *
 * Displays a dialog with all possible column names and allows the user to
 * choose which ones to hide.
 *
 */

import { $ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/checkbox-sk';

export type ColumnTitles = 'Machine' | 'Attached' | 'Device' | 'Mode' | 'Power' | 'Details' | 'Quarantined' | 'Task' | 'Battery' | 'Temperature' | 'Last Seen' | 'Uptime' | 'Dimensions' | 'Launched Swarming' | 'Note' | 'Annotation' | 'Version' | 'Delete';

export const ColumnOrder: ColumnTitles[] = ['Machine', 'Attached', 'Device', 'Mode', 'Power', 'Details', 'Quarantined', 'Task', 'Battery', 'Temperature', 'Last Seen', 'Uptime', 'Dimensions', 'Launched Swarming', 'Note', 'Annotation', 'Version', 'Delete'];

export class MachineTableColumnsDialogSk extends ElementSk {
   private dialog: HTMLDialogElement|null = null;

   private hiddenColumns: ColumnTitles[] = [];

   private resolve: ((value: ColumnTitles[] | undefined)=> void) | null = null;

   constructor() {
     super(MachineTableColumnsDialogSk.template);
   }

  private static template = (ele: MachineTableColumnsDialogSk) => html`
  <dialog>

    <h2>Select columns to display.</h2>
    <div>
      ${ColumnOrder.map((name: ColumnTitles) => html`<checkbox-sk label=${name} ?checked=${!ele.hiddenColumns.includes(name)}></checkbox-sk>`)}
    </div>

    <div class=controls>
      <button @click=${ele.okClick} id=ok>OK</button>
      <button @click=${ele.cancelClick} id=cancel>Cancel</button>
    </div>
  </dialog>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector<HTMLDialogElement>('dialog');
  }

  edit(hidden: ColumnTitles[]): Promise<ColumnTitles[] | undefined> {
    return new Promise((resolve) => {
      this.resolve = resolve;
      this.hiddenColumns = [...hidden];
      this._render();
      this.dialog!.showModal();
    });
  }

  private cancelClick() {
    if (!this.resolve) {
      return;
    }
    this.resolve(undefined);
    this.dialog!.close();
    this.resolve = null;
  }

  private okClick() {
    if (!this.resolve) {
      return;
    }
    this.hiddenColumns = $<CheckOrRadio>('checkbox-sk', this).filter((ch: CheckOrRadio) => !ch.checked).map((ch: CheckOrRadio) => ch.label as ColumnTitles);
    this.resolve(this.hiddenColumns);
    this.dialog!.close();
    this.resolve = null;
  }
}

define('machine-table-columns-dialog-sk', MachineTableColumnsDialogSk);
