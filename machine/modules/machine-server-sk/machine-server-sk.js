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
import { diffDate } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/styles/buttons';

const REFRESH_LOCALSTORAGE_KEY = 'autorefresh';

const temps = (temperatures) => {
  if (!temperatures) {
    return '';
  }
  const values = Object.values(temperatures);
  if (!values.length) {
    return '';
  }
  let total = 0;
  values.forEach((x) => { total += x; });
  const ave = total / values.length;
  return html`
  <details>
    <summary>Avg: ${ave.toFixed(1)}</summary>
    <table>
    ${Object.entries(temperatures).map((pair) => html`<tr><td>${pair[0]}</td><td>${pair[1]}</td></tr>`)}
    </table>
  </details>
  `;
};

const isRunning = (machine) => (machine.RunningSwarmingTask ? html`<cached-icon-sk title="Running"></cached-icon-sk>` : '');

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
${machine.Annotation.User} (${diffDate(machine.Annotation.Timestamp)}) - ${machine.Annotation.Message}
`;
};

const update = (machine) => {
  if (machine.ScheduledForDeletion) {
    return 'Waiting for update.';
  }
  return 'Update';
};

const imageName = (machine) => {
  // KubernetesImage looks like:
  // "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean".
  // We just need to display everything after the ":".
  const parts = machine.KubernetesImage.split(':');
  if (parts.length < 2) {
    return '(missing)';
  }
  return parts[1];
};

const rows = (ele) => ele._machines.map((machine) => html`
<tr id=${machine.Dimensions.id}>
  <td><a href="https://chromium-swarm.appspot.com/bot?id=${machine.Dimensions.id}">${machine.Dimensions.id}</a></td>
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
  <td>${diffDate(machine.LastUpdated)}</td>
  <td>${dimensions(machine)}</td>
  <td>${annotation(machine)}</td>
  <td>${imageName(machine)}</td>
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
  <span
    id=refresh
    @click=${() => ele._toggleRefresh()}
    title="Start/Stop the automatic refreshing of data on the page."
    >${refreshButtonDisplayValue(ele)}</span>
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
