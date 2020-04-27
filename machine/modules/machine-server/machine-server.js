/**
 * @module modules/machine-server
 * @description <h2><code>machine-server</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const rows = (ele) => ele._machines.map((machine) => html`
<tr>
  <td>${machine.Dimensions.id}</td>
  <td>${machine.Dimensions.device_type}</td>
  <td>
    <button>${machine.Mode}</button>
  </td>
  <td>${machine.Dimensions.quarantined}</td>
  <td>${machine.Battery}</td>
  <td>
    ${Object.entries(machine.Temperature).map((pair) => html`<div>${pair[0]}=${pair[1]}</div>`)}
  </td>
  <td>${machine.LastUpdated}</td>
</tr>

`);

const template = (ele) => html`
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
`;

window.customElements.define('machine-server', class extends ElementSk {
  constructor() {
    super(template);
    this._machines = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }
});
