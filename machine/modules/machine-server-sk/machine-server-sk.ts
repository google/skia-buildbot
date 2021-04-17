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

import { errorMessage } from 'elements-sk/errorMessage';
import { diffDate, strDuration } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { $$ } from 'common-sk/modules/dom';
import { Annotation, Description } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import 'elements-sk/error-toast-sk/index';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/clear-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/edit-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/power-settings-new-icon-sk';
import 'elements-sk/styles/buttons/index';
import { NoteEditorSk } from '../note-editor-sk/note-editor-sk';
import '../note-editor-sk';

const REFRESH_LOCALSTORAGE_KEY = 'autorefresh';

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

const temps = (temperatures: { [key: string]: number }): TemplateResult => {
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

const isRunning = (machine: Description): TemplateResult => (machine.RunningSwarmingTask
  ? html`
        <cached-icon-sk title="Running"></cached-icon-sk>
      `
  : html``);

const asList = (arr: string[]) => arr.join(' | ');

const dimensions = (machine: Description): TemplateResult => {
  if (!machine.Dimensions) {
    return html``;
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

const annotation = (ann: Annotation | null): TemplateResult => {
  if (!ann?.Message) {
    return html``;
  }
  return html`
    ${ann.User} (${diffDate(ann.Timestamp)}) -
    ${ann.Message}
  `;
};

// eslint-disable-next-line no-use-before-define
const update = (ele: MachineServerSk, machine: Description): TemplateResult => {
  const msg = machine.ScheduledForDeletion ? 'Waiting for update.' : 'Update';
  return html`
    <button
      title="Force the pod to be killed and re-created"
      class="update"
      @click=${() => ele.toggleUpdate(machine.Dimensions.id![0])}
    >
      ${msg}
    </button>
  `;
};

const imageName = (machine: Description): string => {
  // KubernetesImage looks like:
  // "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean".
  // We just need to display everything after the ":".
  if (!machine.KubernetesImage) {
    return '(missing)';
  }
  const parts = machine.KubernetesImage.split(':');
  if (parts.length < 2) {
    return '(missing)';
  }
  return parts[1];
};

// eslint-disable-next-line no-use-before-define
const powerCycle = (ele: MachineServerSk, machine: Description): TemplateResult => {
  if (machine.PowerCycle) {
    return html`Waiting for Power Cycle`;
  }
  return html`
    <power-settings-new-icon-sk
      title="Powercycle the host"
      @click=${() => ele.togglePowerCycle(machine.Dimensions.id![0])}
    ></power-settings-new-icon-sk>
  `;
};

// eslint-disable-next-line no-use-before-define
const clearDevice = (ele: MachineServerSk, machine: Description): TemplateResult => (machine.RunningSwarmingTask
  ? html``
  : html`
        <clear-icon-sk
          title="Clear the dimensions for the bot"
          @click=${() => ele.clearDevice(machine.Dimensions.id![0])}
        ></clear-icon-sk>
      `);

// eslint-disable-next-line no-use-before-define
const toggleMode = (ele: MachineServerSk, machine: Description) => html`
    <button
      class="mode"
      @click=${() => ele.toggleMode(machine.Dimensions.id![0])}
      title="Put the machine in maintenance mode."
    >
      ${machine.Mode}
    </button>
  `;

const machineLink = (machine: Description): TemplateResult => html`
    <a
      href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions.id}"
    >
      ${machine.Dimensions.id}
    </a>
  `;

// eslint-disable-next-line no-use-before-define
const deleteMachine = (ele: MachineServerSk, machine: Description): TemplateResult => html`
  <delete-icon-sk
    title="Remove the machine from the database."
    @click=${() => ele.deleteDevice(machine.Dimensions.id![0])}
  ></delete-icon-sk>
`;

/** Displays the device uptime, truncated to the minute. */
const deviceUptime = (machine: Description): TemplateResult => html`
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
const note = (ele: MachineServerSk, machine: Description): TemplateResult => html`
  <edit-icon-sk @click=${() => ele.editNote(machine.Dimensions.id![0], machine)}></edit-icon-sk>${annotation(machine.Note)}
`;

// eslint-disable-next-line no-use-before-define
const rows = (ele: MachineServerSk): TemplateResult[] => ele.filteredMachines().map(
  (machine) => html`
      <tr id=${machine.Dimensions.id![0]}>
        <td>${machineLink(machine)}</td>
        <td>${machine.PodName}</td>
        <td>${machine.Dimensions.device_type}</td>
        <td>${toggleMode(ele, machine)}</td>
        <td>${update(ele, machine)}</td>
        <td class="powercycle">${powerCycle(ele, machine)}</td>
        <td>${clearDevice(ele, machine)}</td>
        <td>${machine.Dimensions.quarantined}</td>
        <td>${isRunning(machine)}</td>
        <td>${machine.Battery}</td>
        <td>${temps(machine.Temperature)}</td>
        <td class="${outOfSpecIfTooOld(machine.LastUpdated)}">${diffDate(machine.LastUpdated)}</td>
        <td class="${uptimeOutOfSpecIfTooOld(machine.DeviceUptime)}">${deviceUptime(machine)}</td>
        <td>${dimensions(machine)}</td>
        <td>${note(ele, machine)}</td>
        <td>${annotation(machine.Annotation)}</td>
        <td>${imageName(machine)}</td>
        <td>${deleteMachine(ele, machine)}</td>
      </tr>
    `,
);

// eslint-disable-next-line no-use-before-define
const refreshButtonDisplayValue = (ele: MachineServerSk): TemplateResult => {
  if (ele.refreshing) {
    return html`
      <pause-icon-sk></pause-icon-sk>
    `;
  }
  return html`
    <play-arrow-icon-sk></play-arrow-icon-sk>
  `;
};

// eslint-disable-next-line no-use-before-define
const template = (ele: MachineServerSk): TemplateResult => html`
  <header>
    <span
      id="refresh"
      @click=${() => ele.toggleRefresh()}
      title="Start/Stop the automatic refreshing of data on the page."
    >
      ${refreshButtonDisplayValue(ele)}
    </span>
    <span id=header-rhs>
      <input type="text" placeholder="Filter" @input=${ele.filterInput}>
      <theme-chooser-sk
        title="Toggle between light and dark mode."
      ></theme-chooser-sk>
    </span>
  </header>
  <main>
    <table>
      <tr>
        <th>Machine</th>
        <th>Pod</th>
        <th>Device</th>
        <th>Mode</th>
        <th>Update</th>
        <th>Host</th>
        <th>Device</th>
        <th>Quarantined</th>
        <th>Task</th>
        <th>Battery</th>
        <th>Temperature</th>
        <th>Last Seen</th>
        <th>Uptime</th>
        <th>Dimensions</th>
        <th>Note</th>
        <th>Annotation</th>
        <th>Image</th>
        <th>Delete</th>
      </tr>
      ${rows(ele)}
    </table>
  </main>
  <note-editor-sk></note-editor-sk>
  <error-toast-sk></error-toast-sk>
`;

export class MachineServerSk extends ElementSk {
  machines: Description[] = [];

  machinesAsString: string[] = [];

  timeout: number;

  filter: string = ''

  private noteEditor: NoteEditorSk | null = null;

  constructor() {
    super(template);

    // The id of the running setTimeout, if any, otherwise 0.
    this.timeout = 0;
  }

  filterInput(e: InputEvent): void {
    this.filter = (e.target as HTMLInputElement).value.toLowerCase();
    this._render();
  }

  filteredMachines(): Description[] {
    const ret: Description[] = [];
    this.machinesAsString.forEach((machineAsString, index) => {
      if (machineAsString.includes(this.filter)) {
        ret.push(this.machines[index]);
      }
    });
    return ret;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    this.noteEditor = $$<NoteEditorSk>('note-editor-sk', this)!;
    await this.refreshStep();
  }

  /** @prop refreshing {bool} True if the data on the page is periodically refreshed. */
  get refreshing(): boolean {
    return window.localStorage.getItem(REFRESH_LOCALSTORAGE_KEY) === 'true';
  }

  set refreshing(val: boolean) {
    window.localStorage.setItem(REFRESH_LOCALSTORAGE_KEY, `${!!val}`);
  }

  async toggleUpdate(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_update/${id}`);
      this.removeAttribute('waiting');
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  async toggleMode(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_mode/${id}`);
      this.removeAttribute('waiting');
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  async editNote(id: string, machine: Description): Promise<void> {
    try {
      const editedAnnotation = await this.noteEditor!.edit(machine.Note);
      if (!editedAnnotation) {
        return;
      }
      this.setAttribute('waiting', '');
      const resp = await fetch(`/_/machine/set_note/${id}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(editedAnnotation),
      });
      this.removeAttribute('waiting');
      if (!resp.ok) {
        this.onError(resp.statusText);
      }
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  async togglePowerCycle(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_powercycle/${id}`);
      this.removeAttribute('waiting');
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  async clearDevice(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/remove_device/${id}`);
      this.removeAttribute('waiting');
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  async deleteDevice(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/delete_machine/${id}`);
      this.removeAttribute('waiting');
      await this.update(true);
    } catch (error) {
      this.onError(error);
    }
  }

  toggleRefresh(): void {
    this.refreshing = !this.refreshing;
    this.refreshStep();
  }

  private async refreshStep(): Promise<void> {
    // Wait for _update to finish so we don't pile up requests if server latency
    // rises.
    await this.update();
    if (this.refreshing && this.timeout === 0) {
      this.timeout = window.setTimeout(() => {
        // Only done here, so multiple calls to _refreshStep() won't start
        // parallel setTimeout chains.
        this.timeout = 0;

        this.refreshStep();
      }, 2000);
    }
  }

  private onError(msg: any) {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }

  private async update(changeCursor = false) {
    if (changeCursor) {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch('/_/machines');
      const json = await jsonOrThrow(resp);
      if (changeCursor) {
        this.removeAttribute('waiting');
      }
      this.machines = json;
      this.machinesAsString = this.machines.map((machine) => JSON.stringify(machine).toLowerCase());
      this._render();
    } catch (error) {
      this.onError(error);
    }
  }
}

window.customElements.define('machine-server-sk', MachineServerSk);
