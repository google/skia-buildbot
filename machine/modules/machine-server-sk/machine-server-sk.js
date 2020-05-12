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

import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/pause-icon-sk';

const REFRESH_LOCALSTORAGE_KEY = 'autorefresh';

const temps = (temperatures) => {
  if (!temperatures) {
    return '';
  }
  return Object.entries(temperatures).map((pair) => html`<div>${pair[0]}=${pair[1]}</div>`);
};

const isRunning = (machine) => (machine.RunningSwarmingTask ? html`&check;` : '');

const asList = (arr) => arr.join(' | ');

const dimensions = (machine) => {
  if (!machine.Dimensions) {
    return '';
  }
  return html`
<details>
  <summary>Dimensions</summary>
  <table>
  ${Object.entries(machine.Dimensions).map((pair) => html`<tr><td>${pair[0]}</td><td>${asList(pair[1])}</td></tr>`)}
  </table>
</details>
`;
};

const annotation = (machine) => {
  if (!machine.Annotation.Message) {
    return '';
  }
  return html`
<div>${machine.Annotation.Message}</div>
<div>${machine.Annotation.User}</div>
<div>${machine.Annotation.Timestamp}</div>
`;
};

const update = (machine) => {
  if (machine.ScheduledForDeletion) {
    return 'Waiting for update.';
  }
  return 'Update';
};

const rows = (ele) => ele._machines.map((machine) => html`
<tr id=${machine.Dimensions.id}>
  <td>${machine.Dimensions.id}</td>
  <td>${machine.PodName}</td>
  <td>${machine.Dimensions.device_type}</td>
  <td><button class=mode @click=${() => ele._toggleMode(machine.Dimensions.id)}>${machine.Mode}</button></td>
  <td><button class=update @click=${() => ele._toggleUpdate(machine.Dimensions.id)}>${update(machine)}</button></td>
  <td>${machine.Dimensions.quarantined}</td>
  <td>${isRunning(machine)}</td>
  <td>${machine.Battery}</td>
  <td>
    ${temps(machine.Temperature)}
  </td>
  <td>${machine.LastUpdated}</td>
  <td>${dimensions(machine)}</td>
  <td>${annotation(machine)}</td>
</tr>
`);

const refreshButtonDisplayValue = (ele) => {
  if (ele.refreshing) {
    return html`<pause-icon-sk></pause-icon-sk>`;
  }
  return html`<play-arrow-icon-sk></play-arrow-icon-sk>`;
};

const template = (ele) => html`
<header>
  <h1>Machines</h1>
  <button
    id=refresh
    @click=${() => ele._toggleRefresh()}
    title="Start/Stop the automatic refreshing of data on the page."
    >${refreshButtonDisplayValue(ele)}</button>
</header>
<main>
  <table>
  <tr>
    <th>Machine</th>
    <th>Pod</th>
    <th>Device</th>
    <th>Mode</th>
    <th>Update</th>
    <th>Quarantined</th>
    <th>Running Task</th>
    <th>Battery</th>
    <th>Temperature</th>
    <th>Last Updated</th>
    <th>Dimensions</th>
    <th>Annotation</th>
  </tr>
  ${rows(ele)}
  </table>
</main>
<error-toast-sk></error-toast-sk>
`;

window.customElements.define('machine-server-sk', class extends ElementSk {
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
  get refreshing() { return window.localStorage.getItem(REFRESH_LOCALSTORAGE_KEY) === 'true'; }

  set refreshing(val) { window.localStorage.setItem(REFRESH_LOCALSTORAGE_KEY, !!val); }

  _onError(msg) {
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

  async _toggleMode(id) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_mode/${id}`);
      this.removeAttribute('waiting');
      this._update(true);
    } catch (error) {
      this._onError(error);
    }
  }

  async _toggleUpdate(id) {
    try {
      this.setAttribute('waiting', '');
      await fetch(`/_/machine/toggle_update/${id}`);
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

  disconnectedCallback() {
  }
});
