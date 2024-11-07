/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/checkbox-sk';
import { Anomaly } from '../json';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import '../window/window';

class AnomalyGroup {
  anomalies: Anomaly[] = [];

  expanded: boolean = false;
}

export class AnomaliesTableSk extends ElementSk {
  // TODO(eduardoyap): change to window.perf.bug_host_url.
  private bug_host_url: string = 'b';

  private anomalyList: Anomaly[] = [];

  private anomalyGroups: AnomalyGroup[] = [];

  constructor() {
    super(AnomaliesTableSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private static template = (ele: AnomaliesTableSk) => html` ${ele.generateTable()} `;

  private rangeIntersects(aMin: number, aMax: number, bMin: number, bMax: number) {
    return aMin <= bMax && bMin <= aMax;
  }

  private shouldMerge(a: Anomaly, b: Anomaly) {
    return this.rangeIntersects(a.start_revision, a.end_revision, b.start_revision, b.end_revision);
  }

  /**
   * Merge anomalies into groups.
   *
   * The criteria for merging two anomalies A and B is if A.start_revision and A.end_revision
   * intersect with B.start_revision and B.end_revision.
   */
  private groupAnomalies() {
    const groups = [];

    for (let i = 0; i < this.anomalyList.length; i++) {
      let merged = false;
      const anomaly = this.anomalyList[i];
      for (const group of groups) {
        let doMerge = true;
        for (const other of group.anomalies) {
          const should = this.shouldMerge(anomaly, other);
          if (!should) {
            doMerge = false;
            break;
          }
        }
        if (doMerge) {
          group.anomalies.push(anomaly);
          merged = true;
          break;
        }
      }
      if (!merged) {
        groups.push({
          anomalies: [anomaly],
          expanded: false,
        });
      }
    }
    this.anomalyGroups = groups;
  }

  private generateTable() {
    return html`
      <table>
        <tr>
          <th></th>
          <th>
            <checkbox-sk></checkbox-sk>
          </th>
          <th></th>
          <th>Bug ID</th>
          <th>Revisions</th>
          <th>Main</th>
          <th>Bot</th>
          <th>Test Suite</th>
          <th>Test</th>
          <th>Change Direction</th>
          <th>Delta %</th>
          <th>Abs Delta</th>
        </tr>
        ${this.generateGroups()}
      </table>
    `;
  }

  private generateGroups() {
    const groups = [];
    for (let i = 0; i < this.anomalyGroups.length; i++) {
      const anomalyGroup = this.anomalyGroups[i];
      groups.push(this.generateRows(anomalyGroup));
    }
    return groups;
  }

  private generateRows(anomalyGroup: AnomalyGroup) {
    const rows = [];
    const length = anomalyGroup.anomalies.length;
    for (let i = 0; i < anomalyGroup.anomalies.length; i++) {
      const anomaly = anomalyGroup.anomalies[i];
      rows.push(html`
        <tr ?hidden=${!anomalyGroup.expanded && i !== 0}>
          <td>
            <button
              class="expand-button"
              @click=${() => this.expandGroup(anomalyGroup)}
              ?hidden=${length === 1 || i > 0}>
              ${length}
            </button>
          </td>
          <td>
            <checkbox-sk></checkbox-sk>
          </td>
          <td></td>
          <td>${AnomalySk.formatBug(this.bug_host_url, anomaly.bug_id)}</td>
          <td>${anomaly.start_revision} - ${anomaly.end_revision}</td>
          <td>${anomaly.test_path.split('/')[0]}</td>
          <td>${anomaly.test_path.split('/')[1]}</td>
          <td>${anomaly.test_path.split('/')[2]}</td>
          <td>${anomaly.test_path.split('/')[3]}</td>
          <td>${anomaly.is_improvement}</td>
          <td>
            ${AnomalySk.getPercentChange(
              anomaly.median_before_anomaly,
              anomaly.median_after_anomaly
            )}
          </td>
          <td>
            ${AnomalySk.formatNumber(anomaly.median_after_anomaly - anomaly.median_before_anomaly)}
          </td>
        </tr>
      `);
    }
    return rows;
  }

  private expandGroup(anomalyGroup: AnomalyGroup) {
    anomalyGroup.expanded = !anomalyGroup.expanded;
    this._render();
  }

  populateTable(anomalyList: Anomaly[]) {
    this.anomalyList = anomalyList;
    this.groupAnomalies();
    this._render();
  }
}

define('anomalies-table-sk', AnomaliesTableSk);
