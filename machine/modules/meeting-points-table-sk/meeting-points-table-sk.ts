/**
 * @module modules/meeting-points-table-sk
 * @description <h2><code>meeting-points-table-sk</code></h2>
 *
 * A readout of currently extant Meeting Points
 */
import { html } from 'lit-html';

import { define } from 'elements-sk/define';
import { LiveTableSk } from '../live-table-sk';
import { MeetingPoint } from '../json';

export class MeetingPointsTableSk extends LiveTableSk<MeetingPoint> {
  fetchPath = '/_/meeting_points';

  tableHeaders() {
    return html`
      <th>Pod</th>
      <th>Port</th>
      <th>Username</th>
      <th>Machine</th>
      <th>Last Seen</th>
    `;
  }

  tableRow(meetingPoint: MeetingPoint) {
    return html`
      <tr>
        <td>${meetingPoint.PodName}</td>
        <td>${meetingPoint.Port}</td>
        <td>${meetingPoint.Username}</td>
        <td>${meetingPoint.MachineID}</td>
        <td>${meetingPoint.LastUpdated}</td>
      </tr>
    `;
  }
}

define('meeting-points-table-sk', MeetingPointsTableSk);
