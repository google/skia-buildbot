/**
 * @module modules/pods-table-sk
 * @description <h2><code>pods-table-sk</code></h2>
 *
 * A readout of currently extant switch-pods
 */
import { html } from 'lit-html';

import { define } from 'elements-sk/define';
import { LiveTableSk } from '../live-table-sk';
import { Pod } from '../json';

export class PodsTableSk extends LiveTableSk<Pod> {
  fetchPath = '/_/pods';

  tableHeaders() {
    return html`
      <th>Name</th>
      <th>Last Updated</th>
    `;
  }

  tableRow(pod: Pod) {
    return html`
      <tr>
        <td>${pod.Name}</td>
        <td>${pod.LastUpdated}</td>
      </tr>
    `;
  }
}

define('pods-table-sk', PodsTableSk);
