/**
 * @module modules/machine-server
 * @description <h2><code>machine-server</code></h2>
 *
 * The main machine server landing page.
 *
 * @attr waiting - If present then display the waiting cursor.
 */
import { html } from 'lit-html';

import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/error-toast-sk';

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

const rows = (ele) => ele._machines.map((machine) => html`
<tr id=${machine.Dimensions.id}>
  <td>${machine.Dimensions.id}</td>
  <td>${machine.Dimensions.device_type}</td>
  <td><button @click=${() => ele._toggleMode(machine.Dimensions.id)}>${machine.Mode}</button></td>
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

const template = (ele) => html`
<header>
  <h1>Machines</h1>
</header>
<main>
  <table>
  <tr>
    <th>Machine</th>
    <th>Device</th>
    <th>Mode</th>
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
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._update();
  }

  _onError(msg) {
    this.removeAttribute('waiting');
    errorMessage(msg);
  }

  _update() {
    this.setAttribute('waiting', '');
    this._machines = fetch('/_/machines').then(jsonOrThrow).then((json) => {
      this.removeAttribute('waiting');
      this._machines = json;
      this._render();
    }).catch((msg) => this._onError(msg));
  }

  _toggleMode(id) {
    this.setAttribute('waiting', '');
    this._machines = fetch(`/_/machine/toggle_mode/${id}`).then(() => {
      this.removeAttribute('waiting');
      this._update();
    }).catch((msg) => this._onError(msg));
  }

  disconnectedCallback() {
  }
});
