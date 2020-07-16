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
import { html } from 'lit-html';
import { Description } from '../json';

import { errorMessage } from 'elements-sk/errorMessage';
import { diffDate } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/clear-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/power-settings-new-icon-sk';
import 'elements-sk/styles/buttons';

const REFRESH_LOCALSTORAGE_KEY = 'autorefresh';

const temps = (temperatures: { [key: string]: number }) => {
  if (!temperatures) {
    return '';
  }
  const values = Object.values(temperatures);
  if (!values.length) {
    return '';
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
          (pair) =>
            html`
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

const isRunning = (machine: Description) =>
  machine.RunningSwarmingTask
    ? html`
        <cached-icon-sk title="Running"></cached-icon-sk>
      `
    : '';

const asList = (arr: string[]) => arr.join(' | ');

const dimensions = (machine: Description) => {
  if (!machine.Dimensions) {
    return '';
  }
  return html`
    <details class="dimensions">
      <summary>Dimensions</summary>
      <table>
        ${Object.entries(machine.Dimensions).map(
          (pair) =>
            html`
              <tr>
                <td>${pair[0]}</td>
                <td>${asList(pair[1])}</td>
              </tr>
            `
        )}
      </table>
    </details>
  `;
};

const annotation = (machine: Description) => {
  if (!machine.Annotation.Message) {
    return '';
  }
  return html`
    ${machine.Annotation.User} (${diffDate(machine.Annotation.Timestamp)}) -
    ${machine.Annotation.Message}
  `;
};

const update = (ele: MachineServerSk, machine: Description) => {
  const msg = machine.ScheduledForDeletion ? 'Waiting for update.' : 'Update';
  return html`
    <button
      title="Force the pod to be killed and re-created"
      class="update"
      @click=${() => ele._toggleUpdate(machine.Dimensions.id)}
    >
      ${msg}
    </button>
  `;
};

const imageName = (machine: Description) => {
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

const powerCycle = (ele: MachineServerSk, machine: Description) => {
  if (machine.PowerCycle) {
    return 'Waiting for Power Cycle';
  }
  return html`
    <power-settings-new-icon-sk
      title="Powercycle the host"
      @click=${() => ele._togglePowerCycle(machine.Dimensions.id)}
    ></power-settings-new-icon-sk>
  `;
};

const clearDevice = (ele: MachineServerSk, machine: Description) => {
  return machine.RunningSwarmingTask
    ? ''
    : html`
        <clear-icon-sk
          title="Clear the dimensions for the bot"
          @click=${() => ele._clearDevice(machine.Dimensions.id)}
        ></clear-icon-sk>
      `;
};

const toggleMode = (ele: MachineServerSk, machine: Description) => {
  return html`
    <button class="mode" @click=${() => ele._toggleMode(machine.Dimensions.id)}>
      ${machine.Mode}
    </button>
  `;
};

const machineLink = (machine: Description) => {
  return html`
    <a
      href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions.id}"
    >
      ${machine.Dimensions.id}
    </a>
  `;
};

const rows = (ele: MachineServerSk) =>
  ele._machines.map(
    (machine) => html`
      <tr id=${machine.Dimensions.id}>
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
        <td>${diffDate(machine.LastUpdated)}</td>
        <td>${dimensions(machine)}</td>
        <td>${annotation(machine)}</td>
        <td>${imageName(machine)}</td>
      </tr>
    `
  );

const refreshButtonDisplayValue = (ele: MachineServerSk) => {
  if (ele.refreshing) {
    return html`
      <pause-icon-sk></pause-icon-sk>
    `;
  }
  return html`
    <play-arrow-icon-sk></play-arrow-icon-sk>
  `;
};

const template = (ele: MachineServerSk) => html`
  <header>
    <span
      id="refresh"
      @click=${() => ele._toggleRefresh()}
      title="Start/Stop the automatic refreshing of data on the page."
    >
      ${refreshButtonDisplayValue(ele)}
    </span>
    <theme-chooser-sk
      title="Toggle between light and dark mode."
    ></theme-chooser-sk>
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
        <th>Dimensions</th>
        <th>Annotation</th>
        <th>Image</th>
      </tr>
      ${rows(ele)}
    </table>
  </main>
  <error-toast-sk></error-toast-sk>
`;

export class MachineServerSk extends ElementSk {
  _machines: Description[];

  _timeout: number;

  constructor() {
    super(template);
    this._machines = [];

    // The id of the running setTimeout, if any, otherwise 0.
    this._timeout = 0;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._refreshStep();
  }

  /** @prop refreshing {bool} True if the data on the page is periodically refreshed. */
  get refreshing() {
    return window.localStorage.getItem(REFRESH_LOCALSTORAGE_KEY) === 'true';
  }

  set refreshing(val) {
    window.localStorage.setItem(REFRESH_LOCALSTORAGE_KEY, '' + !!val);
  }

  _onError(msg: object) {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }

  async _update(changeCursor = false) {
    if (changeCursor) {
      this.setAttribute('waiting', '');
    }

    try {
      const resp = await fetch('/_/machines');
      const json = await jsonOrThrow(resp);
      if (changeCursor) {
        this.removeAttribute('waiting');
      }
      this._machines = json;
      this._render();
    } catch (error) {
      this._onError(error);
    }
  }

  async _toggleMode(id: string[]) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_mode/${id}`);
      this.removeAttribute('waiting');
      this._update(true);
    } catch (error) {
      this._onError(error);
    }
  }

  async _toggleUpdate(id: string[]) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_update/${id}`);
      this.removeAttribute('waiting');
      this._update(true);
    } catch (error) {
      this._onError(error);
    }
  }

  async _togglePowerCycle(id: string[]) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_powercycle/${id}`);
      this.removeAttribute('waiting');
      this._update(true);
    } catch (error) {
      this._onError(error);
    }
  }

  async _clearDevice(id: string[]) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/remove_device/${id}`);
      this.removeAttribute('waiting');
      this._update(true);
    } catch (error) {
      this._onError(error);
    }
  }

  async _refreshStep() {
    // Wait for _update to finish so we don't pile up requests if server latency
    // rises.
    await this._update();
    if (this.refreshing && this._timeout === 0) {
      this._timeout = window.setTimeout(() => {
        // Only done here, so multiple calls to _refreshStep() won't start
        // parallel setTimeout chains.
        this._timeout = 0;

        this._refreshStep();
      }, 2000);
    }
  }

  _toggleRefresh() {
    this.refreshing = !this.refreshing;
    this._refreshStep();
  }

  disconnectedCallback() {}
}

window.customElements.define('machine-server-sk', MachineServerSk);
