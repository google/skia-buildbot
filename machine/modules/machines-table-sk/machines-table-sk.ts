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
import { html, TemplateResult } from 'lit/html.js';

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { diffDate, strDuration } from '../../../infra-sk/modules/human';
import { $$ } from '../../../infra-sk/modules/dom';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import {
  Annotation,
  AttachedDevice,
  Description,
  SetAttachedDevice,
  SetNoteRequest,
  SupplyChromeOSRequest,
} from '../json';

import '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import '../../../infra-sk/modules/clipboard-sk';
import '../../../elements-sk/modules/error-toast-sk/index';
import '../../../elements-sk/modules/icons/block-icon-sk';
import '../../../elements-sk/modules/icons/cached-icon-sk';
import '../../../elements-sk/modules/icons/delete-icon-sk';
import '../../../elements-sk/modules/icons/edit-icon-sk';
import '../../../elements-sk/modules/icons/launch-icon-sk';
import '../../../elements-sk/modules/icons/power-settings-new-icon-sk';
import '../../../elements-sk/modules/icons/warning-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/icons/sort-icon-sk';
import '../../../elements-sk/modules/icons/arrow-drop-down-icon-sk';
import '../../../elements-sk/modules/icons/arrow-drop-up-icon-sk';
import '../../../elements-sk/modules/icons/clear-all-icon-sk';
import '../../../elements-sk/modules/icons/content-copy-icon-sk';
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
import {
  ColumnOrder,
  ColumnTitles,
  MachineTableColumnsDialogSk,
} from '../machine-table-columns-dialog-sk/machine-table-columns-dialog-sk';
import '../machine-table-columns-dialog-sk/machine-table-columns-dialog-sk';

export type WaitCursor = 'DoNotShowWaitCursor' | 'ShowWaitCursor';

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

export const MachineTableSkSortChangeEventName: string =
  'machine-table-sort-change';

/** The event detail is the sort history of the table encoded as a string. */
export type MachineTableSkChangeEventDetail = string;

const attachedDeviceDisplayName: Record<string, AttachedDevice> = {
  '-': 'nodevice',
  Android: 'adb',
  iOS: 'ios',
  pyocd: 'pyocd',
  SSH: 'ssh',
};

/** attachedDeviceDisplayName keys sorted by display name. */
const attachedDeviceDisplayNamesOrder: string[] = Object.keys(
  attachedDeviceDisplayName
).sort();

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

/** checkResponse throws an error with the response body text if the response
 * was not 'ok'. */
const checkResponse = async (resp: Response): Promise<void> => {
  if (!resp.ok) {
    throw await resp.text();
  }
};

// Sort functions for different clumns, i.e. values in Description.

// Mode no longer exists, so this is a no-op.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const sortByMode = (a: Description, b: Description): number =>
  a.MaintenanceMode.localeCompare(b.MaintenanceMode);

export const sortByAttachedDevice = (a: Description, b: Description): number =>
  a.AttachedDevice.localeCompare(b.AttachedDevice);

export const sortByAnnotation = (a: Description, b: Description): number =>
  a.Annotation.Message.localeCompare(b.Annotation.Message);

export const sortByNote = (a: Description, b: Description): number =>
  a.Note.Message.localeCompare(b.Note.Message);

export const sortByVersion = (a: Description, b: Description): number =>
  a.Version.localeCompare(b.Version);

export const sortByPowerCycle = (a: Description, b: Description): number => {
  const powerCycleSort = sortBooleans(a.PowerCycle, b.PowerCycle);
  if (powerCycleSort === 0) {
    return a.PowerCycleState.localeCompare(b.PowerCycleState);
  }
  return powerCycleSort;
};

export const sortByLastUpated = (a: Description, b: Description): number =>
  a.LastUpdated.localeCompare(b.LastUpdated);

export const sortByBattery = (a: Description, b: Description): number =>
  a.Battery - b.Battery;

export const sortByRunningSwarmingTask = (
  a: Description,
  b: Description
): number => sortBooleans(a.RunningSwarmingTask, b.RunningSwarmingTask);

export const sortByLaunchedSwarming = (
  a: Description,
  b: Description
): number => sortBooleans(a.LaunchedSwarming, b.LaunchedSwarming);

export const sortByDeviceUptime = (a: Description, b: Description): number =>
  a.DeviceUptime - b.DeviceUptime;

export const sortByDevice = (a: Description, b: Description): number =>
  pretty_device_name_as_string(a).localeCompare(
    pretty_device_name_as_string(b)
  );

export const sortByQuarantined = (a: Description, b: Description): number => {
  const qa = a.Dimensions!.quarantined?.join('') || '';
  const qb = b.Dimensions!.quarantined?.join('') || '';
  return qa.localeCompare(qb);
};

export const sortByMachineID = (a: Description, b: Description): number => {
  const qa = a.Dimensions!.id?.join('') || '';
  const qb = b.Dimensions!.id?.join('') || '';
  return qa.localeCompare(qb);
};

export const sortByRecovering = (a: Description, b: Description): number =>
  a.Recovering.localeCompare(b.Recovering);

export const sortByIsQuarantined = (a: Description, b: Description): number =>
  sortBooleans(a.IsQuarantined, b.IsQuarantined);

// Do not change the location of these functions, i.e. their index, as that would
// change the meaning of URLs already in the wild. Always add new sort functions
// to the end of the list, and if a sort function is no-longer used replace it with
// a no-op function, e.g. (a: Description, b: Description): number => 0.
const sortFunctionsByColumn: compareFunc<Description>[] = [
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
  sortByRecovering,
  sortByIsQuarantined,
];

const temps = (machine: Description): TemplateResult => {
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
          `
        )}
      </table>
    </details>
  `;
};

const lastSeen = (machine: Description): TemplateResult =>
  html`${diffDate(machine.LastUpdated)}`;

const isRunning = (machine: Description): TemplateResult =>
  machine.RunningSwarmingTask
    ? html` <cached-icon-sk title="Running"></cached-icon-sk> `
    : html``;

const asList = (arr: string[] | null) => (arr === null ? '' : arr.join(' | '));

const launchedSwarming = (machine: Description): TemplateResult => {
  if (!machine.LaunchedSwarming) {
    return html``;
  }
  return html`
    <launch-icon-sk
      title="Swarming was launched by test_machine_monitor."></launch-icon-sk>
  `;
};

const annotation = (ann: Annotation): TemplateResult => {
  if (!ann?.Message) {
    return html``;
  }
  return html` ${ann.User} (${diffDate(ann.Timestamp)}) - ${ann.Message} `;
};

const imageVersion = (machine: Description): TemplateResult => {
  if (machine.Version) {
    return html`${machine.Version}`;
  }
  return html`(missing)`;
};

/** Displays the device uptime, truncated to the minute. */
const deviceUptime = (machine: Description): TemplateResult => html`
  ${strDuration(machine.DeviceUptime - (machine.DeviceUptime % 60))}
`;

/** Returns the CSS class that should decorate the LastUpdated value. */
export const outOfSpecIfTooOld = (machine: Description): string => {
  const lastUpdated = machine.LastUpdated;
  const diff = Date.now() - Date.parse(lastUpdated);
  return diff > MAX_LAST_UPDATED_ACCEPTABLE_MS ? 'outOfSpec' : '';
};

/** Returns the CSS class that should decorate the Uptime value. */
export const uptimeOutOfSpecIfTooOld = (machine: Description): string =>
  machine.DeviceUptime > MAX_UPTIME_ACCEPTABLE_S ? 'outOfSpec' : '';

// Returns the device_type separated with vertical bars and a trailing device
// alias if that name is known.
const pretty_device_name = (machine: Description): TemplateResult =>
  html`${pretty_device_name_as_string(machine)}`;

// Returns the device_type separated with vertical bars and a trailing device
// alias if that name is known.
export const pretty_device_name_as_string = (machine: Description): string => {
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

const quarantined = (machine: Description): TemplateResult =>
  html`${machine.Dimensions!.quarantined}`;

const battery = (machine: Description): TemplateResult =>
  html`${machine.Battery}`;

// Column stores information about a single column in the table.
class Column {
  name: string;

  row: (machine: Description) => TemplateResult;

  compare: compareFunc<Description> | null;

  className: ((machine: Description) => string) | null;

  computedClipValue: (() => Promise<string>) | null;

  /** constructor
   *
   * name - The displayed name of the column.
   * row - A function that emits the `td` HTML for each row in this column,
   * compare - An optional comparison function for sorting based on the values in this column.
   * className - An optional function to compute the class name to apply to each row's `td`.
   * computedClipValue - An optional function that computes a value to send to the clipboard. A non-null
   *    value will cause a clipboard-sk element to be added to the header.
   */
  constructor(
    name: string,
    row: (machine: Description) => TemplateResult,
    compare: compareFunc<Description> | null,
    className: ((machine: Description) => string) | null = null,
    computedClipValue: (() => Promise<string>) | null = null
  ) {
    this.name = name;
    this.row = row;
    this.compare = compare;
    this.className = className;
    this.computedClipValue = computedClipValue;
  }

  // eslint-disable-next-line no-use-before-define
  header(ele: MachinesTableSk): TemplateResult {
    return html`<th>${
      this.name
    }${this.optionalClipboard()}${this.optionalSortArrow(ele)}</div></th>`;
  }

  // eslint-disable-next-line no-use-before-define
  optionalSortArrow(ele: MachinesTableSk): TemplateResult {
    if (this.compare === null) {
      return html``;
    }
    return html`&nbsp;${ele.sortArrow(this.compare)}`;
  }

  optionalClipboard(): TemplateResult {
    if (this.computedClipValue === null) {
      return html``;
    }
    return html`&nbsp;<clipboard-sk
        .calculatedValue=${this.computedClipValue}></clipboard-sk>`;
  }

  rowValue(machine: Description): TemplateResult {
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

  private sortHistory: SortHistory<Description> = new SortHistory(
    sortFunctionsByColumn
  );

  private filterer: FilterArray<Description> = new FilterArray();

  private hiddenColumns: ColumnTitles[] = [
    'Launched Swarming',
    'Version',
    'Annotation',
  ];

  private hiddenColumnsDialog: MachineTableColumnsDialogSk | null = null;

  private columns: Record<ColumnTitles, Column> | null = null;

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
        this.machineLink.bind(this),
        sortByMachineID,
        null,
        this.allDisplayedMachineIDs.bind(this)
      ),
      Attached: new Column(
        'Attached',
        this.attachedDevice.bind(this),
        sortByAttachedDevice
      ),
      Device: new Column('Device', pretty_device_name, sortByDevice),
      Mode: new Column('Mode', this.toggleModeElement.bind(this), sortByMode),
      Recovering: new Column(
        'Recovering',
        this.recovering.bind(this),
        sortByRecovering
      ),
      Power: new Column(
        'Power',
        this.powerCycle.bind(this),
        sortByPowerCycle,
        () => 'powercycle'
      ),
      Details: new Column('Details', this.editDeviceIcon.bind(this), null),
      Quarantined: new Column('Quarantined', quarantined, sortByQuarantined),
      'Clear Quarantine': new Column(
        'Clear Quarantine',
        this.clearQuarantine.bind(this),
        sortByIsQuarantined
      ),
      Task: new Column('Task', isRunning, sortByRunningSwarmingTask),
      Battery: new Column('Battery', battery, sortByBattery),
      Temperature: new Column('Temperature', temps, null),
      'Last Seen': new Column(
        'Last Seen',
        lastSeen,
        sortByLastUpated,
        outOfSpecIfTooOld
      ),
      Uptime: new Column(
        'Uptime',
        deviceUptime,
        sortByDeviceUptime,
        uptimeOutOfSpecIfTooOld
      ),
      Dimensions: new Column('Dimensions', this.dimensions.bind(this), null),
      'Launched Swarming': new Column(
        'Launched Swarming',
        launchedSwarming,
        sortByLaunchedSwarming
      ),
      Note: new Column('Note', this.note.bind(this), sortByNote),
      Annotation: new Column(
        'Annotation',
        (machine: Description) => annotation(machine.Annotation),
        sortByAnnotation
      ),
      Version: new Column('Version', imageVersion, sortByVersion),
      Delete: new Column('Delete', this.deleteMachine.bind(this), null),
    };
  }

  async allDisplayedMachineIDs(): Promise<string> {
    return this.orderedFilteredRows()
      .map((d: Description) => d.Dimensions!.id![0])
      .join('\n');
  }

  private orderedFilteredRows(): Description[] {
    const ret = this.filterer.matchingValues();
    ret.sort(this.sortHistory.compare.bind(this.sortHistory));
    return ret;
  }

  private tableRows(): TemplateResult[] {
    const ret: TemplateResult[] = [];
    this.orderedFilteredRows().forEach((item) =>
      ret.push(
        html`<tr>
          ${this.tableRow(item)}
        </tr>`
      )
    );
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
  toggleModeElement(machine: Description): TemplateResult {
    return html`
      <button
        class="mode"
        @click=${() => this.toggleMode(machine.Dimensions!.id![0])}
        title="${machine.MaintenanceMode || 'Put machine in maintenance mode'}">
        ${machine.MaintenanceMode === '' ? 'available' : 'maintenance'}
      </button>
    `;
  }

  recovering(machine: Description): TemplateResult {
    return html`${machine.Recovering}`;
  }

  powerCycle(machine: Description): TemplateResult {
    return html`
      <power-settings-new-icon-sk
        title="Powercycle the host"
        class="clickable"
        @click=${() => this.togglePowerCycle(machine.Dimensions!.id![0])}
        ?hidden=${machine.PowerCycleState !==
        'available'}></power-settings-new-icon-sk>
      <warning-icon-sk
        ?hidden=${machine.PowerCycleState !== 'in_error'}
        title="Controller failed to connect."></warning-icon-sk>
      <spinner-sk ?active=${machine.PowerCycle}></spinner-sk>
    `;
  }

  clearQuarantine(machine: Description): TemplateResult {
    return html`
      <block-icon-sk
        title="Clear the quarantine"
        class="clickable"
        @click=${() => this.clearQuarantineAction(machine.Dimensions!.id![0])}
        ?hidden=${!machine.IsQuarantined}></block-icon-sk>
    `;
  }

  editDeviceIcon(machine: Description): TemplateResult {
    return machine.RunningSwarmingTask || machine.AttachedDevice !== 'ssh'
      ? html``
      : html`
          <edit-icon-sk
            title="Edit/clear the dimensions for the bot"
            class="edit_device"
            @click=${() =>
              this.deviceEditor!.show(
                machine.Dimensions,
                machine.SSHUserIP
              )}></edit-icon-sk>
        `;
  }

  note(machine: Description): TemplateResult {
    return html`
      <edit-icon-sk
        class="edit_note clickable"
        @click=${() =>
          this.editNote(machine.Dimensions!.id![0], machine)}></edit-icon-sk
      >${annotation(machine.Note)}
    `;
  }

  deleteMachine(machine: Description): TemplateResult {
    return html`
      <delete-icon-sk
        title="Remove the machine from the database."
        class="clickable"
        @click=${() =>
          this.deleteDevice(machine.Dimensions!.id![0])}></delete-icon-sk>
    `;
  }

  dimensions(machine: Description): TemplateResult {
    if (!machine.Dimensions) {
      return html`<div>Unknown</div>`;
    }
    return html`
      <div class="dimensions">
        <clear-all-icon-sk
          title="Clear all dimensions from the datastore"
          @click=${() => this.clearDeviceByID(machine.Dimensions!.id![0])}>
        </clear-all-icon-sk>
        <details class="dimensions">
          <summary>Dimensions</summary>
          <table>
            ${Object.entries(machine.Dimensions).map(
              (pair) => html`
                <tr>
                  <td>${pair[0]}</td>
                  <td>${asList(pair[1])}</td>
                </tr>
              `
            )}
          </table>
        </details>
      </div>
    `;
  }

  /**
   * Fetch the latest list from the server, and update the page to reflect it.
   *
   * @param showWaitCursor Whether the mouse pointer should be changed to a
   *   spinner while we wait for the fetch
   */
  async update(
    waitCursorPolicy: WaitCursor = 'DoNotShowWaitCursor'
  ): Promise<void> {
    if (waitCursorPolicy === 'ShowWaitCursor') {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch(fetchPath);
      await checkResponse(resp);
      const json = await jsonOrThrow(resp);
      if (waitCursorPolicy === 'ShowWaitCursor') {
        this.removeAttribute('waiting');
      }
      // Add the pretty device name to each machine description so that the filterer will take it
      // into account. The added field is not used anywhere else.
      const jsonWithPrettyDeviceNames = (json as Description[]).map(
        (machine: Description) => ({
          ...machine,
          __prettyDeviceName__: pretty_device_name_as_string(machine),
        })
      );
      this.filterer.updateArray(jsonWithPrettyDeviceNames);
      this._render();
    } catch (error: any) {
      this.onError(error);
    }
  }

  onError(msg: { message: string } | string): void {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }

  tableHeaders(): TemplateResult[] {
    return ColumnOrder.filter((name) => !this.hiddenColumns.includes(name)).map(
      (columnName) => this.columns![columnName].header(this)
    );
  }

  tableRow(machine: Description): TemplateResult[] {
    if (!machine.Dimensions || !machine.Dimensions.id) {
      return [];
    }
    return ColumnOrder.filter((name) => !this.hiddenColumns.includes(name)).map(
      (columnName) => this.columns![columnName].rowValue(machine)
    );
  }

  private moreTemplate(): TemplateResult {
    return html`
      <note-editor-sk></note-editor-sk>
      <device-editor-sk></device-editor-sk>
      <machine-table-columns-dialog-sk></machine-table-columns-dialog-sk>
    `;
  }

  private attachedDeviceOptions(machine: Description): TemplateResult[] {
    return attachedDeviceDisplayNamesOrder.map(
      (key: string) =>
        html` <option
          value=${attachedDeviceDisplayName[key]}
          ?selected=${attachedDeviceDisplayName[key] ===
          machine.AttachedDevice}>
          ${key}
        </option>`
    );
  }

  private machineLink(machine: Description): TemplateResult {
    return html`
      <a
        href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions!
          .id}">
        ${machine.Dimensions!.id}
      </a>
      <clipboard-sk value="${machine.Dimensions!.id![0]}"></clipboard-sk>
    `;
  }

  private attachedDevice(machine: Description): TemplateResult {
    return html` <select
      @input=${(e: InputEvent) =>
        this.attachedDeviceChanged(e, machine.Dimensions!.id![0])}>
      ${this.attachedDeviceOptions(machine)}
    </select>`;
  }

  sortArrow(fn: compareFunc<Description>): TemplateResult {
    const column = sortFunctionsByColumn.indexOf(fn);
    if (column === -1) {
      errorMessage(`Invalid compareFunc: ${fn.name}`);
    }
    const firstSortSelection = this.sortHistory!.history[0];

    if (column === firstSortSelection.column) {
      if (firstSortSelection.dir === up) {
        return html`<arrow-drop-up-icon-sk
          title="Change sort order to descending."
          @click=${() => this.changeSort(column)}></arrow-drop-up-icon-sk>`;
      }
      return html`<arrow-drop-down-icon-sk
        title="Change sort order to ascending."
        @click=${() => this.changeSort(column)}></arrow-drop-down-icon-sk>`;
    }
    return html`<sort-icon-sk
      title="Sort this column."
      @click=${() => this.changeSort(column)}></sort-icon-sk>`;
  }

  private changeSort(column: number) {
    this.sortHistory!.selectColumnToSortOn(column);

    this.dispatchEvent(
      new CustomEvent<MachineTableSkChangeEventDetail>(
        MachineTableSkSortChangeEventName,
        { detail: this.sortHistory!.encode(), bubbles: true }
      )
    );
    this._render();
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.noteEditor = $$<NoteEditorSk>('note-editor-sk', this);
    this.deviceEditor = $$<DeviceEditorSk>('device-editor-sk', this);
    this.hiddenColumnsDialog = $$<MachineTableColumnsDialogSk>(
      'machine-table-columns-dialog-sk',
      this
    );

    this.addEventListener(ClearDeviceEvent, this.clearDevice);
    this.addEventListener(UpdateDimensionsEvent, this.updateDimensions);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();

    this.removeEventListener(ClearDeviceEvent, this.clearDevice);
    this.removeEventListener(UpdateDimensionsEvent, this.updateDimensions);
  }

  /** Performs the fetch and if successful updates the UI. */
  async fetchCheckAndUpdate(input: string, init?: RequestInit): Promise<void> {
    try {
      this.setAttribute('waiting', '');
      const resp = await fetch(input, init);
      await checkResponse(resp);
      await this.update('ShowWaitCursor');
    } catch (error) {
      this.onError(error as string);
    } finally {
      this.removeAttribute('waiting');
    }
  }

  async toggleUpdate(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/toggle_update/${id}`);
  }

  async attachedDeviceChanged(e: InputEvent, id: string): Promise<void> {
    const sel = e.target as HTMLSelectElement;
    const request: SetAttachedDevice = {
      AttachedDevice: sel.selectedOptions[0].value as AttachedDevice,
    };
    await this.fetchCheckAndUpdate(`/_/machine/set_attached_device/${id}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });
  }

  async toggleMode(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/toggle_mode/${id}`, {
      method: 'POST',
    });
  }

  async editNote(id: string, machine: Description): Promise<void> {
    const editedAnnotation = await this.noteEditor!.edit(machine.Note);
    if (!editedAnnotation) {
      return;
    }
    const request: SetNoteRequest = editedAnnotation;
    this.setAttribute('waiting', '');
    await this.fetchCheckAndUpdate(`/_/machine/set_note/${id}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });
  }

  async editHiddenColumns(): Promise<ColumnTitles[]> {
    const newHiddenColumns = await this.hiddenColumnsDialog!.edit(
      this.hiddenColumns
    );
    if (!newHiddenColumns) {
      return this.hiddenColumns;
    }
    this.restoreHiddenColumns(newHiddenColumns);
    return newHiddenColumns;
  }

  async togglePowerCycle(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/toggle_powercycle/${id}`, {
      method: 'POST',
    });
  }

  async clearQuarantineAction(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/clear_quarantined/${id}`, {
      method: 'POST',
    });
  }

  private async clearDevice(e: Event): Promise<void> {
    const id = (e as CustomEvent<string>).detail;
    this.clearDeviceByID(id);
  }

  private async clearDeviceByID(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/remove_device/${id}`, {
      method: 'POST',
    });
  }

  async deleteDevice(id: string): Promise<void> {
    await this.fetchCheckAndUpdate(`/_/machine/delete_machine/${id}`, {
      method: 'POST',
    });
  }

  private async updateDimensions(e: Event): Promise<void> {
    const info = (e as CustomEvent<UpdateDimensionsDetails>).detail;
    const postBody: SupplyChromeOSRequest = {
      SSHUserIP: info.sshUserIP,
      SuppliedDimensions: info.specifiedDimensions,
    };
    await this.fetchCheckAndUpdate(
      `/_/machine/supply_chromeos/${info.machineID}`,
      {
        method: 'POST',
        body: JSON.stringify(postBody),
      }
    );
  }
}

window.customElements.define('machines-table-sk', MachinesTableSk);
