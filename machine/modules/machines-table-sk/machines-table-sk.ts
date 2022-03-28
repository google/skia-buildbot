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

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { diffDate, strDuration } from 'common-sk/modules/human';
import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import {
  Annotation,
  AttachedDevice, FrontendDescription, SetAttachedDevice, SetNoteRequest, SupplyChromeOSRequest,
} from '../json';

import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import 'elements-sk/error-toast-sk/index';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/edit-icon-sk';
import 'elements-sk/icon/launch-icon-sk';
import 'elements-sk/icon/power-settings-new-icon-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/spinner-sk';
import 'elements-sk/icon/sort-icon-sk';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';
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
import { compareFunc, SortHistory, up } from '../sort';
import { ElementSk } from '../../../infra-sk/modules/ElementSk/ElementSk';
import { FilterArray } from '../filter-array';
import { ColumnOrder, ColumnTitles, MachineTableColumnsDialogSk } from '../machine-table-columns-dialog-sk/machine-table-columns-dialog-sk';
import '../machine-table-columns-dialog-sk/machine-table-columns-dialog-sk';

export type WaitCursor = 'DoNotShowWaitCursor' | 'ShowWaitCursor'

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

export const MachineTableSkSortChangeEventName: string = 'machine-table-sort-change';

/** The event detail is the sort history of the table encoded as a string. */
export type MachineTableSkChangeEventDetail = string;

const attachedDeviceDisplayName: Record< string, AttachedDevice> = {
  '-': 'nodevice',
  Android: 'adb',
  iOS: 'ios',
  SSH: 'ssh',
};

/** attachedDeviceDisplayName keys sorted by display name. */
const attachedDeviceDisplayNamesOrder: string[] = Object.keys(attachedDeviceDisplayName).sort();

/** sortBooleans is a utility function for sorting booleans, where true comes
 * before false. */
const sortBooleans = (a: boolean, b: boolean): number => {
  if (a === b) {
    return 0;
  }
  if (a) {
    return 1;
  }
  return -1;
};

// Sort functions for different clumns, i.e. values in FrontendDescription.
export const sortByMode = (a: FrontendDescription, b: FrontendDescription): number => a.Mode.localeCompare(b.Mode);

export const sortByAttachedDevice = (a: FrontendDescription, b: FrontendDescription): number => a.AttachedDevice.localeCompare(b.AttachedDevice);

export const sortByAnnotation = (a: FrontendDescription, b: FrontendDescription): number => a.Annotation.Message.localeCompare(b.Annotation.Message);

export const sortByNote = (a: FrontendDescription, b: FrontendDescription): number => a.Note.Message.localeCompare(b.Note.Message);

export const sortByVersion = (a: FrontendDescription, b: FrontendDescription): number => a.Version.localeCompare(b.Version);

export const sortByPowerCycle = (a: FrontendDescription, b: FrontendDescription): number => sortBooleans(a.PowerCycle, b.PowerCycle);

export const sortByLastUpated = (a: FrontendDescription, b: FrontendDescription): number => a.LastUpdated.localeCompare(b.LastUpdated);

export const sortByBattery = (a: FrontendDescription, b: FrontendDescription): number => a.Battery - b.Battery;

export const sortByRunningSwarmingTask = (a: FrontendDescription, b: FrontendDescription): number => sortBooleans(a.RunningSwarmingTask, b.RunningSwarmingTask);

export const sortByLaunchedSwarming = (a: FrontendDescription, b: FrontendDescription): number => sortBooleans(a.LaunchedSwarming, b.LaunchedSwarming);

export const sortByDeviceUptime = (a: FrontendDescription, b: FrontendDescription): number => a.DeviceUptime - b.DeviceUptime;

export const sortByDevice = (a: FrontendDescription, b: FrontendDescription): number => pretty_device_name_as_string(a).localeCompare(pretty_device_name_as_string(b));

export const sortByQuarantined = (a: FrontendDescription, b: FrontendDescription): number => {
  const qa = a.Dimensions!.quarantined?.join('') || '';
  const qb = b.Dimensions!.quarantined?.join('') || '';
  return qa.localeCompare(qb);
};

export const sortByMachineID = (a: FrontendDescription, b: FrontendDescription): number => {
  const qa = a.Dimensions!.id?.join('') || '';
  const qb = b.Dimensions!.id?.join('') || '';
  return qa.localeCompare(qb);
};

// Do not change the location of these functions, i.e. their index, as that would
// change the meaning of URLs already in the wild. Always add new sort functions
// to the end of the list, and if a sort function is no-longer used replace it with
// a no-op function, e.g. (a: FrontendDescription, b: FrontendDescription): number => 0.
const sortFunctionsByColumn: compareFunc<FrontendDescription>[] = [
  sortByMachineID,
  sortByAttachedDevice,
  sortByDevice,
  sortByMode,
  sortByPowerCycle,
  sortByQuarantined,
  sortByRunningSwarmingTask,
  sortByBattery,
  sortByLastUpated,
  sortByDeviceUptime,
  sortByLaunchedSwarming,
  sortByNote,
  sortByAnnotation,
  sortByVersion,
];

const temps = (machine: FrontendDescription): TemplateResult => {
  const temperatures = machine.Temperature;
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

const lastSeen = (machine: FrontendDescription): TemplateResult => html`${diffDate(machine.LastUpdated)}`;

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

const annotation = (ann: Annotation): TemplateResult => {
  if (!ann?.Message) {
    return html``;
  }
  return html`
    ${ann.User} (${diffDate(ann.Timestamp)}) -
    ${ann.Message}
  `;
};

const imageVersion = (machine: FrontendDescription): TemplateResult => {
  if (machine.Version) {
    return html`${machine.Version}`;
  }
  return html`(missing)`;
};

const machineLink = (machine: FrontendDescription): TemplateResult => html`
    <a
      href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions!.id}"
    >
      ${machine.Dimensions!.id}
    </a>
  `;

/** Displays the device uptime, truncated to the minute. */
const deviceUptime = (machine: FrontendDescription): TemplateResult => html`
  ${strDuration(machine.DeviceUptime - (machine.DeviceUptime % 60))}
`;

/** Returns the CSS class that should decorate the LastUpdated value. */
export const outOfSpecIfTooOld = (machine: FrontendDescription): string => {
  const lastUpdated = machine.LastUpdated;
  const diff = (Date.now() - Date.parse(lastUpdated));
  return diff > MAX_LAST_UPDATED_ACCEPTABLE_MS ? 'outOfSpec' : '';
};

/** Returns the CSS class that should decorate the Uptime value. */
export const uptimeOutOfSpecIfTooOld = (machine: FrontendDescription): string => (machine.DeviceUptime > MAX_UPTIME_ACCEPTABLE_S ? 'outOfSpec' : '');

// Returns the device_type separated with vertical bars and a trailing device
// alias if that name is known.
const pretty_device_name = (machine: FrontendDescription): TemplateResult => html`${pretty_device_name_as_string(machine)}`;

// Returns the device_type separated with vertical bars and a trailing device
// alias if that name is known.
export const pretty_device_name_as_string = (machine: FrontendDescription): string => {
  const devices = machine.Dimensions?.device_type;

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

const quarantined = (machine: FrontendDescription): TemplateResult => html`${machine.Dimensions!.quarantined}`;

const battery = (machine: FrontendDescription): TemplateResult => html`${machine.Battery}`;

// Column stores information about a single column in the table.
class Column {
  name: string;

  row: (machine: FrontendDescription)=> TemplateResult;

  compare: compareFunc<FrontendDescription> | null;

  className: ((machine: FrontendDescription)=> string) | null;

  constructor(name: string, row: (machine: FrontendDescription)=> TemplateResult, compare: compareFunc<FrontendDescription> | null, className: ((machine: FrontendDescription)=> string) | null = null) {
    this.name = name;
    this.row = row;
    this.compare = compare;
    this.className = className;
  }

  // eslint-disable-next-line no-use-before-define
  header(ele: MachinesTableSk): TemplateResult {
    if (this.compare !== null) {
      return html`<th>${this.name}&nbsp;${ele.sortArrow(this.compare)}</div></th>`;
    }
    return html`<th>${this.name}</th>`;
  }

  rowValue(machine: FrontendDescription): TemplateResult {
    if (this.className === null) {
      return html`<td>${this.row(machine)}</td>`;
    }
    return html`<td class=${this.className(machine)}>${this.row(machine)}</td>`;
  }
}

/**
   * The URL path from which to fetch the JSON representation of the latest
   * list items
   */
const fetchPath = '/_/machines';

export class MachinesTableSk extends ElementSk {
  private noteEditor: NoteEditorSk | null = null;

  deviceEditor: DeviceEditorSk | null = null;

  private sortHistory: SortHistory<FrontendDescription> = new SortHistory(sortFunctionsByColumn);

  private filterer: FilterArray<FrontendDescription> = new FilterArray();

  private hiddenColumns: ColumnTitles[] = ['Launched Swarming', 'Version', 'Annotation'];

  private hiddenColumnsDialog: MachineTableColumnsDialogSk | null = null;

  private columns: Record<ColumnTitles, Column> | null = null

  private static template = (ele: MachinesTableSk): TemplateResult => html`
    <table>
      <thead>
        <tr>
          ${ele.tableHeaders()}
        </tr>
      </thead>
      <tbody>
        ${ele.tableRows()}
      </tbody>
    </table>
    ${ele.moreTemplate()}
  `;

  constructor() {
    super(MachinesTableSk.template);
    this.classList.add('defaultLiveTableSkStyling');

    this.columns = {
      Machine: new Column(
        'Machine',
        machineLink,
        sortByMachineID,
      ),
      Attached: new Column(
        'Attached',
        this.attachedDevice.bind(this),
        sortByAttachedDevice,
      ),
      Device: new Column(
        'Device',
        pretty_device_name,
        sortByDevice,
      ),
      Mode: new Column(
        'Mode',
        this.toggleModeElement.bind(this),
        sortByMode,
      ),
      Power: new Column(
        'Power',
        this.powerCycle.bind(this),
        sortByPowerCycle,
        () => 'powercycle',
      ),
      Details: new Column(
        'Details',
        this.editDeviceIcon.bind(this),
        null,
      ),
      Quarantined: new Column(
        'Quarantined',
        quarantined,
        sortByQuarantined,
      ),
      Task: new Column(
        'Task',
        isRunning,
        sortByRunningSwarmingTask,
      ),
      Battery: new Column(
        'Battery',
        battery,
        sortByBattery,
      ),
      Temperature: new Column(
        'Temperature',
        temps,
        null,
      ),
      'Last Seen': new Column(
        'Last Seen',
        lastSeen,
        sortByLastUpated,
        outOfSpecIfTooOld,
      ),
      Uptime: new Column(
        'Uptime',
        deviceUptime,
        sortByDeviceUptime,
        uptimeOutOfSpecIfTooOld,
      ),
      Dimensions: new Column(
        'Dimensions',
        dimensions,
        null,
      ),
      'Launched Swarming': new Column(
        'Launched Swarming',
        launchedSwarming,
        sortByLaunchedSwarming,
      ),
      Note: new Column(
        'Note',
        this.note.bind(this),
        sortByNote,
      ),
      Annotation: new Column(
        'Annotation',
        (machine: FrontendDescription) => annotation(machine.Annotation),
        sortByAnnotation,
      ),
      Version: new Column(
        'Version',
        imageVersion,
        sortByVersion,
      ),
      Delete: new Column(
        'Delete',
        this.deleteMachine.bind(this),
        null,
      ),
    };
  }

  private tableRows(): TemplateResult[] {
    const values = this.filterer.matchingValues();
    values.sort(this.sortHistory.compare.bind(this.sortHistory));
    const ret: TemplateResult[] = [];
    values.forEach((item) => ret.push(html`<tr>${this.tableRow(item)}</tr>`));
    return ret;
  }

  /**
   * Show and hide rows to reflect a change in the filtration string.
   */
  filterChanged(value: string): void {
    this.filterer.filterChanged(value);
    this._render();
  }

  restoreSortState(value: string): void {
    this.sortHistory.decode(value);
  }

  restoreHiddenColumns(value: ColumnTitles[]): void {
    this.hiddenColumns = value;
    this._render();
  }

  // eslint-disable-next-line no-use-before-define
  toggleModeElement(machine: FrontendDescription): TemplateResult {
    return html`
    <button
      class="mode"
      @click=${() => this.toggleMode(machine.Dimensions!.id![0])}
      title="Put the machine in maintenance mode."
    >
      ${machine.Mode}
    </button>
  `;
  }

  powerCycle(machine: FrontendDescription): TemplateResult {
    return html`
    <power-settings-new-icon-sk
      title="Powercycle the host"
      class="clickable"
      @click=${() => this.togglePowerCycle(machine.Dimensions!.id![0])}
    ></power-settings-new-icon-sk>
    <spinner-sk ?active=${machine.PowerCycle}></spinner-sk>
  `;
  }

  editDeviceIcon(machine: FrontendDescription): TemplateResult {
    return ((machine.RunningSwarmingTask || machine.AttachedDevice !== 'ssh')
      ? html``
      : html`
        <edit-icon-sk
          title="Edit/clear the dimensions for the bot"
          class="edit_device"
          @click=${() => this.deviceEditor!.show(machine.Dimensions, machine.SSHUserIP)}
        ></edit-icon-sk>
        `);
  }

  note(machine: FrontendDescription): TemplateResult {
    return html`
      <edit-icon-sk
          class="edit_note clickable"
          @click=${() => this.editNote(machine.Dimensions!.id![0], machine)}></edit-icon-sk>${annotation(machine.Note)}
          `;
  }

  deleteMachine(machine: FrontendDescription): TemplateResult {
    return html`
      <delete-icon-sk
        title="Remove the machine from the database."
        class="clickable"
        @click=${() => this.deleteDevice(machine.Dimensions!.id![0])}
      ></delete-icon-sk>
      `;
  }

  /**
   * Fetch the latest list from the server, and update the page to reflect it.
   *
   * @param showWaitCursor Whether the mouse pointer should be changed to a
   *   spinner while we wait for the fetch
   */
  async update(waitCursorPolicy: WaitCursor = 'DoNotShowWaitCursor'): Promise<void> {
    if (waitCursorPolicy === 'ShowWaitCursor') {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch(fetchPath);
      const json = await jsonOrThrow(resp);
      if (waitCursorPolicy === 'ShowWaitCursor') {
        this.removeAttribute('waiting');
      }
      this.filterer.updateArray(json);
      this._render();
    } catch (error: any) {
      this.onError(error);
    }
  }

  onError(msg: { message: string; } | string): void {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }

  tableHeaders(): TemplateResult[] {
    return ColumnOrder.filter((name) => !this.hiddenColumns.includes(name)).map((columnName) => this.columns![columnName].header(this));
  }

  tableRow(machine: FrontendDescription): TemplateResult[] {
    if (!machine.Dimensions || !machine.Dimensions.id) {
      return [];
    }
    return ColumnOrder.filter((name) => !this.hiddenColumns.includes(name)).map((columnName) => this.columns![columnName].rowValue(machine));
  }

  private moreTemplate(): TemplateResult {
    return html`
      <note-editor-sk></note-editor-sk>
      <device-editor-sk></device-editor-sk>
      <machine-table-columns-dialog-sk></machine-table-columns-dialog-sk>
    `;
  }

  private attachedDeviceOptions(machine: FrontendDescription): TemplateResult[] {
    return attachedDeviceDisplayNamesOrder.map((key: string) => html`
      <option
        value=${attachedDeviceDisplayName[key]}
        ?selected=${attachedDeviceDisplayName[key] === machine.AttachedDevice}>
        ${key}
      </option>`);
  }

  private attachedDevice(machine: FrontendDescription): TemplateResult {
    return html`
    <select
      @input=${(e: InputEvent) => this.attachedDeviceChanged(e, machine.Dimensions!.id![0])}>
      ${this.attachedDeviceOptions(machine)}
    </select>`;
  }

  sortArrow(fn: compareFunc<FrontendDescription>): TemplateResult {
    const column = sortFunctionsByColumn.indexOf(fn);
    if (column === -1) {
      errorMessage(`Invalid compareFunc: ${fn.name}`);
    }
    const firstSortSelection = this.sortHistory!.history[0];

    if (column === firstSortSelection.column) {
      if (firstSortSelection.dir === up) {
        return html`<arrow-drop-up-icon-sk title="Change sort order to descending." @click=${() => this.changeSort(column)}></arrow-drop-up-icon-sk>`;
      }
      return html`<arrow-drop-down-icon-sk title="Change sort order to ascending." @click=${() => this.changeSort(column)}></arrow-drop-down-icon-sk>`;
    }
    return html`<sort-icon-sk title="Sort this column." @click=${() => this.changeSort(column)}></sort-icon-sk>`;
  }

  private changeSort(column: number) {
    this.sortHistory!.selectColumnToSortOn(column);

    this.dispatchEvent(
      new CustomEvent<MachineTableSkChangeEventDetail>(
        MachineTableSkSortChangeEventName, { detail: this.sortHistory!.encode(), bubbles: true },
      ),
    );
    this._render();
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.noteEditor = $$<NoteEditorSk>('note-editor-sk', this);
    this.deviceEditor = $$<DeviceEditorSk>('device-editor-sk', this);
    this.hiddenColumnsDialog = $$<MachineTableColumnsDialogSk>('machine-table-columns-dialog-sk', this);

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
      await this.update('ShowWaitCursor');
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
      await this.update('ShowWaitCursor');
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
      await this.update('ShowWaitCursor');
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
      await this.update('ShowWaitCursor');
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  async editHiddenColumns(): Promise<ColumnTitles[]> {
    const newHiddenColumns = await this.hiddenColumnsDialog!.edit(this.hiddenColumns);
    if (!newHiddenColumns) {
      return this.hiddenColumns;
    }
    this.restoreHiddenColumns(newHiddenColumns);
    return newHiddenColumns;
  }

  async togglePowerCycle(id: string): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_powercycle/${id}`, { method: 'POST' });
      await this.update('ShowWaitCursor');
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

      await this.update('ShowWaitCursor');
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
      await this.update('ShowWaitCursor');
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
      await this.update('ShowWaitCursor');
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }
}

window.customElements.define('machines-table-sk', MachinesTableSk);
