/**
 * @module modules/machine-server
 * @description <h2><code>machine-server</code></h2>
 *
 * The main machine server landing page.
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

const rows = (ele) => ele._machines.map((machine) => html`
<tr>
  <td>${machine.Dimensions.id}</td>
  <td>${machine.Dimensions.device_type}</td>
  <td>${machine.Mode}</td>
  <td>${machine.Dimensions.quarantined}</td>
  <td>${machine.Battery}</td>
  <td>
    ${temps(machine.Temperature)}
  </td>
  <td>${machine.LastUpdated}</td>
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
    <th>Battery</th>
    <th>Temperature</th>
    <th>Last Updated</th>
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

  _update() {
    this._machines = fetch('/_/machines').then(jsonOrThrow).then((json) => {
      this._machines = json;
      this._render();
    }).catch(errorMessage);
  }


  disconnectedCallback() {
  }
});
