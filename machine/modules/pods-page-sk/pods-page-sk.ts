/**
 * @module modules/pods-page-sk
 * @description <h2><code>pods-page-sk</code></h2>
 *
 * A readout of currently extant switch-pods
 */
import { html } from 'lit-html';

import { define } from 'elements-sk/define';
import { ListPageSk } from '../list-page-sk';
import { Pod } from '../json';

export class PodsPageSk extends ListPageSk<Pod> {
  _fetchPath = '/_/pods';

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
};

define('pods-page-sk', PodsPageSk);
