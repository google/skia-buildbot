/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html, TemplateResult } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/checkbox-sk/';
import { Anomaly } from '../json';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import '../window/window';
import { TriageMenuSk } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import '@material/web/button/outlined-button.js';

class AnomalyGroup {
  anomalies: Anomaly[] = [];

  expanded: boolean = false;
}

export class AnomaliesTableSk extends ElementSk {
  // TODO(eduardoyap): change to window.perf.bug_host_url.
  private bug_host_url: string = 'b';

  private anomalyList: Anomaly[] = [];

  private anomalyGroups: AnomalyGroup[] = [];

  private showPopup: boolean = false;

  private checkedAnomaliesSet: Set<Anomaly> = new Set<Anomaly>();

  private triageMenu: TriageMenuSk | null = null;

  private headerCheckbox: CheckOrRadio | null = null;

  private sortBy: string | null = null;

  constructor() {
    super(AnomaliesTableSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();
    this.sortBy = 'end_revision';
    this._render();

    this.triageMenu = this.querySelector('#triage-menu');
    this.triageMenu!.disableNudge();
    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
    this.headerCheckbox = document.getElementById('header-checkbox') as CheckOrRadio;
    document.addEventListener('click', (e: Event) => {
      const triageButton = this.querySelector('#triage-button');
      const popup = this.querySelector('.popup');
      if (this.showPopup && !popup!.contains(e.target as Node) && e.target !== triageButton) {
        this.showPopup = false;
        this._render();
      }
    });
  }

  private static template = (ele: AnomaliesTableSk) => html`
    <button
      id="triage-button"
      @click="${ele.togglePopup}"
      ?disabled="${ele.checkedAnomaliesSet.size === 0}">
      Triage
    </button>
    <div class="popup-container" ?hidden="${!ele.showPopup}">
      <div class="popup">
        <triage-menu-sk id="triage-menu"></triage-menu-sk>
      </div>
    </div>
    ${ele.generateTable()}
  `;

  private togglePopup() {
    this.showPopup = !this.showPopup;
    if (this.showPopup) {
      const triageMenu = this.querySelector('#triage-menu') as TriageMenuSk;
      triageMenu.setAnomalies(Array.from(this.checkedAnomaliesSet), [], []);
    }
    this._render();
  }

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
      <table id="anomalies-table">
        <tr class="headers">
          <th></th>
          <th>
            <checkbox-sk id="header-checkbox" @change=${this.toggleAllCheckboxes}> </checkbox-sk>
          </th>
          <th
            id="graph_header"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Graph
          </th>
          <th
            id="bug_id"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Bug ID
          </th>
          <th
            id="end_revision"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Revisions
          </th>
          <th
            id="master"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Master
          </th>
          <th
            id="bot"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Bot
          </th>
          <th
            id="testsuite"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Test Suite
          </th>
          <th
            id="test"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Test
          </th>
          <th id="change_direction">Change Direction</th>
          <th
            id="percent_changed"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Delta %
          </th>
          <th
            id="absolute_delta"
            @click=${() => {
              this.columnHeaderClicked();
            }}>
            Abs Delta
          </th>
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

  private anomalyChecked(chkbox: CheckOrRadio, a: Anomaly) {
    if (chkbox.checked === true) {
      this.checkedAnomaliesSet.add(a);
      if (this.checkedAnomaliesSet.size === this.anomalyList.length) {
        this.headerCheckbox!.checked = true;
      }
    } else {
      this.headerCheckbox!.checked = false;
      this.checkedAnomaliesSet.delete(a);
    }
    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
    this._render();
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
            <checkbox-sk
              @change=${(e: Event) => {
                // If we just need to check 1 anomaly, just mark it as checked.
                if (i !== 0 || anomalyGroup.anomalies.length === 1 || anomalyGroup.expanded) {
                  this.anomalyChecked(e.target as CheckOrRadio, anomaly);
                } else {
                  // If the top anomaly in a group gets checked and the
                  // group is not expanded, check all children anomalies.
                  this.toggleChildrenCheckboxes(anomalyGroup);
                }
              }}
              id="anomaly-row-${anomaly.id}">
            </checkbox-sk>
          </td>
          <td></td>
          <!--TODO(jiaxindong) update graph link to real dashboard link-->
          <td>
            <trending-up-icon-sk></trending-up-icon-sk>
          </td>
          <!--TODO(jiaxindong) update key value to anomaly id in the group-report link-->
          <td>${AnomalySk.formatBug(this.bug_host_url, anomaly.bug_id)}</td>
          <td>
            <span>${this.computeRevisionRange(anomaly.start_revision, anomaly.end_revision)}</span>
          </td>
          <td>${anomaly.test_path.split('/')[0]}</td>
          <td>${anomaly.test_path.split('/')[1]}</td>
          <td>${anomaly.test_path.split('/')[2]}</td>
          <td>${anomaly.test_path.split('/')[3]}</td>
          <td>${anomaly.is_improvement}</td>
          ${this.getDirectionSign(anomaly.median_before_anomaly, anomaly.median_after_anomaly)}
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

  /**
   * Callback for the click event for a column header.
   * @param {Event} event Clicked event.
   * @param {Object} detail Detail Object.
   */
  private columnHeaderClicked(): void {
    this.sort();
  }

  // TODO(jiaxindong)
  // b/375640853 Group anomalies and sort with the revision range in either high or low direction
  /**
   * Sorts the alert list according to the current values of the properties
   * sortDirection and sortBy.
   */
  private sort() {}

  private computeRevisionRange(start: number | null, end: number | null): string {
    if (start === null || end === null) {
      return '';
    }
    if (start === end) {
      return '' + end;
    }
    return start + ' - ' + end;
  }

  private getDirectionSign(medianBefore: number, medianAfter: number): TemplateResult {
    if (medianBefore < medianAfter) {
      return html`<td><trending-up-icon-sk></trending-up-icon-sk></td>`;
    }
    return html`<td><trending-down-icon-sk></trending-down-icon-sk></td>`;
  }

  populateTable(anomalyList: Anomaly[]) {
    this.anomalyList = anomalyList;
    this.groupAnomalies();
    this._render();
  }

  /**
   * Toggles the checked state of all child checkboxes within an anomaly group when the
   * group is collapsed. This allows the user to check/uncheck all children anomalies
   * at once by interacting with the parent checkbox.
   */
  private toggleChildrenCheckboxes(anomalyGroup: AnomalyGroup) {
    let checked = true;
    anomalyGroup.anomalies.forEach((anomaly, index) => {
      const checkbox = this.querySelector(
        `checkbox-sk[id="anomaly-row-${anomaly.id}"]`
      ) as CheckOrRadio;
      if (index === 0) {
        checked = checkbox.checked;
      } else {
        checkbox.checked = checked;
      }
      this.anomalyChecked(checkbox, anomaly);
    });
    this._render();
  }

  /**
   * Toggles the 'checked' state of all checkboxes in the table based on the state of
   * the header checkbox. This provides a convenient way to select or deselect all
   * anomalies at once.
   */
  private toggleAllCheckboxes() {
    const checked = this.headerCheckbox!.checked;
    this.anomalyGroups.forEach((group) => {
      group.anomalies.forEach((anomaly) => {
        const checkbox = this.querySelector(
          `checkbox-sk[id="anomaly-row-${anomaly.id}"]`
        ) as CheckOrRadio;
        checkbox!.checked = checked;
        this.anomalyChecked(checkbox, anomaly);
      });
    });
    this._render();
  }

  getCheckedAnomalies(): Anomaly[] {
    return Array.from(this.checkedAnomaliesSet);
  }
}

define('anomalies-table-sk', AnomaliesTableSk);
