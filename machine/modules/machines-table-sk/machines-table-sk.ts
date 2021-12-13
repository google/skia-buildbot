/**
 * @module modules/machine-server
 * @description <h2><code>machine-server</code></h2>
 *
 * The main machine server landing page.
 *
 * Uses local storage to persist the user's choice of auto-refresh.
 *
 * @attr waiting - If present then display the waiting cursor.
 */
import { html, TemplateResult } from 'lit-html';

import { diffDate, strDuration } from 'common-sk/modules/human';
import { $$ } from 'common-sk/modules/dom';
import {
  Annotation, AttachedDevice, FrontendDescription, SetAttachedDevice, SetNoteRequest, SupplyChromeOSRequest,
} from '../json';
import { LiveTableSk, WaitCursor } from '../live-table-sk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import 'elements-sk/error-toast-sk/index';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/edit-icon-sk';
import 'elements-sk/icon/launch-icon-sk';
import 'elements-sk/icon/power-settings-new-icon-sk';
import 'elements-sk/styles/buttons/index';
import { NoteEditorSk } from '../note-editor-sk/note-editor-sk';
import '../auto-refresh-sk';
import '../device-editor-sk';
import '../note-editor-sk';
import { DEVICE_ALIASES } from '../../../modules/devices/devices';
import {
  ClearDeviceEvent,
  DeviceEditorSk,
  UpdateDimensionsDetails,
  UpdateDimensionsEvent,
} from '../device-editor-sk/device-editor-sk';

/**
 * Updates should arrive every 30 seconds, so we allow up to 2x that for lag
 * before showing it as an error.
 * */
export const MAX_LAST_UPDATED_ACCEPTABLE_MS = 60 * 1000;

/**
 * Devices should be restarted every 24 hours, with an hour added if they are
 * running a test.
 */
export const MAX_UPTIME_ACCEPTABLE_S = 60 * 60 * 25;

const attachedDeviceDisplayName: Record< string, AttachedDevice> = {
  '-': 'nodevice',
  Android: 'adb',
  iOS: 'ios',
  SSH: 'ssh',
};

// Sorted by display name.
const attachedDeviceDisplayNamesOrder: string[] = Object.keys(attachedDeviceDisplayName).sort();

const temps = (temperatures: { [key: string]: number } | null): TemplateResult => {
  if (!temperatures) {
    return html``;
  }
  const values = Object.values(temperatures);
  if (!values.length) {
    return html``;
  }
  let total = 0;
  values.forEach((x) => {
    total += x;
  });
  const ave = total / values.length;
  return html`
    <details>
      <summary>Avg: ${ave.toFixed(1)}</summary>
      <table>
        ${Object.entries(temperatures).map(
    (pair) => html`
              <tr>
                <td>${pair[0]}</td>
                <td>${pair[1]}</td>
              </tr>
            `,
  )}
      </table>
    </details>
  `;
};

const isRunning = (machine: FrontendDescription): TemplateResult => (machine.RunningSwarmingTask
  ? html`
        <cached-icon-sk title="Running"></cached-icon-sk>
      `
  : html``);

const asList = (arr: string[]) => arr.join(' | ');

const dimensions = (machine: FrontendDescription): TemplateResult => {
  if (!machine.Dimensions) {
    return html`<div>Unknown</div>`;
  }
  return html`
    <details class="dimensions">
      <summary>Dimensions</summary>
      <table>
        ${Object.entries(machine.Dimensions).map(
    (pair) => html`
              <tr>
                <td>${pair[0]}</td>
                <td>${asList(pair[1]!)}</td>
              </tr>
            `,
  )}
      </table>
    </details>
  `;
};

const launchedSwarming = (machine: FrontendDescription): TemplateResult => {
  if (!machine.LaunchedSwarming) {
    return html``;
  }
  return html`
    <launch-icon-sk title="Swarming was launched by test_machine_monitor."></launch-icon-sk>
  `;
};

const annotation = (ann: Annotation | null): TemplateResult => {
  if (!ann?.Message) {
    return html``;
  }
  return html`
    ${ann.User} (${diffDate(ann.Timestamp)}) -
    ${ann.Message}
  `;
};

const imageVersion = (machine: FrontendDescription): string => {
  if (machine.Version) {
    return machine.Version;
  }
  return '(missing)';
};

// eslint-disable-next-line no-use-before-define
const powerCycle = (ele: MachinesTableSk, machine: FrontendDescription): TemplateResult => {
  if (machine.PowerCycle) {
    return html`Waiting for Power Cycle`;
  }
  return html`
    <power-settings-new-icon-sk
      title="Powercycle the host"
      class="clickable"
      @click=${() => ele.togglePowerCycle(machine.Dimensions!.id![0])}
    ></power-settings-new-icon-sk>
  `;
};

// eslint-disable-next-line no-use-before-define
const toggleMode = (ele: MachinesTableSk, machine: FrontendDescription) => html`
    <button
      class="mode"
      @click=${() => ele.toggleMode(machine.Dimensions!.id![0])}
      title="Put the machine in maintenance mode."
    >
      ${machine.Mode}
    </button>
  `;

const machineLink = (machine: FrontendDescription): TemplateResult => html`
    <a
      href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions!.id}"
    >
      ${machine.Dimensions!.id}
    </a>
  `;

// eslint-disable-next-line no-use-before-define
const deleteMachine = (ele: MachinesTableSk, machine: FrontendDescription): TemplateResult => html`
  <delete-icon-sk
    title="Remove the machine from the database."
    class="clickable"
    @click=${() => ele.deleteDevice(machine.Dimensions!.id![0])}
  ></delete-icon-sk>
`;

/** Displays the device uptime, truncated to the minute. */
const deviceUptime = (machine: FrontendDescription): TemplateResult => html`
  ${strDuration(machine.DeviceUptime - (machine.DeviceUptime % 60))}
`;

/** Returns the CSS class that should decorate the LastUpdated value. */
export const outOfSpecIfTooOld = (lastUpdated: string): string => {
  const diff = (Date.now() - Date.parse(lastUpdated));
  return diff > MAX_LAST_UPDATED_ACCEPTABLE_MS ? 'outOfSpec' : '';
};

/** Returns the CSS class that should decorate the Uptime value. */
export const uptimeOutOfSpecIfTooOld = (uptime: number): string => (uptime > MAX_UPTIME_ACCEPTABLE_S ? 'outOfSpec' : '');

// eslint-disable-next-line no-use-before-define
const note = (ele: MachinesTableSk, machine: FrontendDescription): TemplateResult => html`
  <edit-icon-sk
      class="edit_note clickable"
      @click=${() => ele.editNote(machine.Dimensions!.id![0], machine)}></edit-icon-sk>${annotation(machine.Note)}
`;

// Returns the device_type separated with vertical bars and a trailing device
// alias if that name is known.
export const pretty_device_name = (devices: string[] | null): string => {
  if (!devices) {
    return '';
  }
  let alias = '';
  for (let i = 0; i < devices.length; i++) {
    const found = DEVICE_ALIASES[devices[i]];
    if (found) {
      alias = `(${found})`;
    }
  }
  return `${devices.join(' | ')} ${alias}`;
};

const attachedDeviceOption = (key: string, ele: MachinesTableSk, machine: FrontendDescription): TemplateResult => html`<option value=${attachedDeviceDisplayName[key]} ?selected=${attachedDeviceDisplayName[key] === machine.AttachedDevice}>${key}</option>`;

const attachedDevice = (ele: MachinesTableSk, machine: FrontendDescription): TemplateResult => html`
  <select @input=${(e: InputEvent) => ele.attachedDeviceChanged(e, machine.Dimensions!.id![0])}>
    ${attachedDeviceDisplayNamesOrder.map((key: string) => attachedDeviceOption(key, ele, machine))}
  </select>
`;

export class MachinesTableSk extends LiveTableSk<FrontendDescription> {
  private noteEditor: NoteEditorSk | null = null;

  private deviceEditor: DeviceEditorSk | null = null;

  fetchPath = '/_/machines';

  tableHeaders(): TemplateResult {
    return html`
      <th>Machine</th>
      <th>Attached</th>
      <th>Device</th>
      <th>Mode</th>
      <th>Host</th>
      <th>Device</th>
      <th>Quarantined</th>
      <th>Task</th>
      <th>Battery</th>
      <th>Temperature</th>
      <th>Last Seen</th>
      <th>Uptime</th>
      <th>Dimensions</th>
      <th>Launched Swarming</th>
      <th>Note</th>
      <th>Annotation</th>
      <th>Version</th>
      <th>Delete</th>
    `;
  }

  tableRow(machine: FrontendDescription): TemplateResult {
    if (!machine.Dimensions || !machine.Dimensions.id) {
      return html``;
    }
    return html`
      <tr id=${machine.Dimensions.id[0]}>
        <td>${machineLink(machine)}</td>
        <td>${attachedDevice(this, machine)}</td>
        <td>${pretty_device_name(machine.Dimensions.device_type)}</td>
        <td>${toggleMode(this, machine)}</td>
        <td class="powercycle">${powerCycle(this, machine)}</td>
        <td>${this.editDeviceIcon(machine)}</td>
        <td>${machine.Dimensions.quarantined}</td>
        <td>${isRunning(machine)}</td>
        <td>${machine.Battery}</td>
        <td>${temps(machine.Temperature)}</td>
        <td class="${outOfSpecIfTooOld(machine.LastUpdated)}">${diffDate(machine.LastUpdated)}</td>
        <td class="${uptimeOutOfSpecIfTooOld(machine.DeviceUptime)}">${deviceUptime(machine)}</td>
        <td>${dimensions(machine)}</td>
        <td class="center">${launchedSwarming(machine)}</td>
        <td>${note(this, machine)}</td>
        <td>${annotation(machine.Annotation)}</td>
        <td>${imageVersion(machine)}</td>
        <td>${deleteMachine(this, machine)}</td>
      </tr>
    `;
  }

  protected moreTemplate(): TemplateResult {
    return html`
      <note-editor-sk></note-editor-sk>
      <device-editor-sk></device-editor-sk>
    `;
  }

  private editDeviceIcon = (machine: FrontendDescription): TemplateResult => ((machine.RunningSwarmingTask || machine.AttachedDevice !== 'ssh')
    ? html``
    : html`
        <edit-icon-sk
          title="Edit/clear the dimensions for the bot"
          class="edit_device"
          @click=${() => this.deviceEditor?.show(machine.Dimensions, machine.SSHUserIP)}
        ></edit-icon-sk>
      `);

  async connectedCallback(): Promise<void> {
    await super.connectedCallback();
    this.noteEditor = $$<NoteEditorSk>('note-editor-sk', this);
    this.deviceEditor = $$<DeviceEditorSk>('device-editor-sk', this);

    this.addEventListener(ClearDeviceEvent, this.clearDevice);
    this.addEventListener(UpdateDimensionsEvent, this.updateDimensions);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();

    this.removeEventListener(ClearDeviceEvent, this.clearDevice);
    this.removeEventListener(UpdateDimensionsEvent, this.updateDimensions);
  }

  async toggleUpdate(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_update/${id}`);
      this.removeAttribute('waiting');
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    }
  }

  async attachedDeviceChanged(e: InputEvent, id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      const sel = e.target as HTMLSelectElement;
      const request: SetAttachedDevice = {
        AttachedDevice: sel.selectedOptions[0].value as AttachedDevice,
      };
      await fetch(`/_/machine/set_attached_device/${id}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  async toggleMode(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_mode/${id}`, { method: 'POST' });
      this.removeAttribute('waiting');
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    }
  }

  async editNote(id: string, machine: FrontendDescription): Promise<void> {
    try {
      const editedAnnotation = await this.noteEditor!.edit(machine.Note);
      if (!editedAnnotation) {
        return;
      }
      const request: SetNoteRequest = editedAnnotation;
      this.setAttribute('waiting', '');
      const resp = await fetch(`/_/machine/set_note/${id}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });
      if (!resp.ok) {
        this.onError(resp.statusText);
      }
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  async togglePowerCycle(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_powercycle/${id}`, { method: 'POST' });
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  private async clearDevice(e: Event): Promise<void> {
    const id = (e as CustomEvent<string>).detail;
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/remove_device/${id}`, { method: 'POST' });

      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  async deleteDevice(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/delete_machine/${id}`, { method: 'POST' });
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  private async updateDimensions(e: Event): Promise<void> {
    const info = (e as CustomEvent<UpdateDimensionsDetails>).detail;
    const postBody: SupplyChromeOSRequest = {
      SSHUserIP: info.sshUserIP,
      SuppliedDimensions: info.specifiedDimensions,
    };
    try {
      this.setAttribute('waiting', '');

      await fetch(`/_/machine/supply_chromeos/${info.machineID}`, {
        method: 'POST',
        body: JSON.stringify(postBody),
      });
      await this.update(WaitCursor.SHOW);
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }
}

window.customElements.define('machines-table-sk', MachinesTableSk);
