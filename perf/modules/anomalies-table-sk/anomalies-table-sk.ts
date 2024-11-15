/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html, TemplateResult } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/checkbox-sk';
import '../../../infra-sk/modules/sort-sk';
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
  private bug_host_url: string = window.perf.bug_host_url;

  private anomalyList: Anomaly[] = [];

  private anomalyGroups: AnomalyGroup[] = [];

  private showPopup: boolean = false;

  private checkedAnomaliesSet: Set<Anomaly> = new Set<Anomaly>();

  private triageMenu: TriageMenuSk | null = null;

  private headerCheckbox: CheckOrRadio | null = null;

  constructor() {
    super(AnomaliesTableSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();
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
    <h1 id="clear-msg" hidden>All anomalies are triaged!</h1>
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
      <sort-sk id="as_table" target="rows">
        <table id="anomalies-table" hidden>
          <tr class="headers">
            <th id="group"></th>
            <th id="checkbox">
              <checkbox-sk id="header-checkbox" @change=${this.toggleAllCheckboxes}> </checkbox-sk>
            </th>
            <th id="graph_header"></th>
            <th id="bug_id" data-key="bugid">Bug ID</th>
            <th id="revision_range" data-key="revisions" data-default="up">Revisions</th>
            <th id="master" data-key="master" data-sort-type="alpha">Main</th>
            <th id="bot" data-key="bot" data-sort-type="alpha">Bot</th>
            <th id="testsuite" data-key="testsuite" data-sort-type="alpha">Test Suite</th>
            <th id="test" data-key="test" data-sort-type="alpha">Test</th>
            <th id="change_direction" data-key="direction">Change Direction</th>
            <th id="percent_changed" data-key="delta">Delta %</th>
            <th id="absolute_delta" data-key="absdelta">Abs Delta</th>
          </tr>
          <tbody id="rows">
            ${this.generateGroups()}
          </tbody>
        </table>
      </sort-sk>
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

  private getProcessedAnomaly(anomaly: Anomaly) {
    const bugId = anomaly.bug_id;
    const testPathPieces = anomaly.test_path.split('/');
    const master = testPathPieces[0];
    const bot = testPathPieces[1];
    const testsuite = testPathPieces[2];
    const test = testPathPieces.slice(3, testPathPieces.length).join('/');
    const revision = anomaly.end_revision;
    const direction = anomaly.median_before_anomaly - anomaly.median_after_anomaly;
    const delta = AnomalySk.getPercentChange(
      anomaly.median_before_anomaly,
      anomaly.median_after_anomaly
    );
    const absDelta = anomaly.median_after_anomaly - anomaly.median_before_anomaly;
    return {
      bugId,
      revision,
      master,
      bot,
      testsuite,
      test,
      direction,
      delta,
      absDelta,
    };
  }

  private generateRows(anomalyGroup: AnomalyGroup) {
    const rows = [];
    const length = anomalyGroup.anomalies.length;

    const anomalySortValues = this.getProcessedAnomaly(anomalyGroup.anomalies[0]);
    for (let i = 0; i < anomalyGroup.anomalies.length; i++) {
      const anomaly = anomalyGroup.anomalies[i];
      const processedAnomaly = this.getProcessedAnomaly(anomaly);
      const anomalyClass = anomaly.is_improvement ? 'improvement' : 'regression';
      rows.push(html`
        <tr
          data-bugid="${anomalySortValues.bugId}"
          data-revisions="${anomalySortValues.revision}"
          data-master="${anomalySortValues.master}"
          data-bot="${anomalySortValues.bot}"
          data-testsuite="${anomalySortValues.testsuite}"
          data-test="${anomalySortValues.test}"
          data-direction=${anomalySortValues.direction}
          data-delta="${anomalySortValues.delta}"
          data-absdelta="${anomalySortValues.absDelta}"
          class=${this.getRowClass(i, anomalyGroup)}
          ?hidden=${!anomalyGroup.expanded && i !== 0}>
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
                if (i !== 0 || length === 1 || anomalyGroup.expanded) {
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
          <!--TODO(jiaxindong) update graph link to real dashboard link-->
          <td>
            <trending-up-icon-sk></trending-up-icon-sk>
          </td>
          <td>
            ${AnomalySk.formatBug(this.bug_host_url, anomaly.bug_id)}
            <close-icon-sk
              id="btnUnassociate"
              @click=${() => {
                this.triageMenu!.makeEditAnomalyRequest([anomaly], [], 'RESET');
              }}
              ?hidden=${anomaly!.bug_id === 0}>
            </close-icon-sk>
          </td>
          <!--TODO(jiaxindong) update key value to anomaly id in the group-report link-->
          <td>
            <span>${this.computeRevisionRange(anomaly.start_revision, anomaly.end_revision)}</span>
          </td>
          <td>${processedAnomaly.master}</td>
          <td>${processedAnomaly.bot}</td>
          <td>${processedAnomaly.testsuite}</td>
          <td>${processedAnomaly.test}</td>
          <td class=${anomalyClass}>
            ${this.getDirectionSign(anomaly.median_before_anomaly, anomaly.median_after_anomaly)}
          </td>
          <td class=${anomalyClass}>${AnomalySk.formatPercentage(processedAnomaly.delta)}%</td>
          <td class=${anomalyClass}>
            ${AnomalySk.formatNumber(processedAnomaly.absDelta)} ${anomaly.units}
          </td>
        </tr>
      `);
    }
    return rows;
  }

  private getRowClass(index: number, anomalyGroup: AnomalyGroup) {
    if (anomalyGroup.expanded) {
      if (index === 0) {
        return 'parent-expanded-row';
      } else {
        return 'child-expanded-row';
      }
    }
    return '';
  }

  private expandGroup(anomalyGroup: AnomalyGroup) {
    anomalyGroup.expanded = !anomalyGroup.expanded;
    this._render();
  }

  private computeRevisionRange(start: number | null, end: number | null): string {
    if (start === null || end === null) {
      return '';
    }
    if (start === end) {
      return '' + end;
    }
    return start + ' - ' + end;
  }

  // return up or down triangle.
  // also suppressed the 'Non ASCII character found' error.
  private getDirectionSign(medianBefore: number, medianAfter: number): TemplateResult {
    if (medianBefore <= medianAfter) {
      return html`\u25B2`; // prettier-ignore
    }
    return html`\u25BC`; // prettier-ignore
  }

  populateTable(anomalyList: Anomaly[]) {
    const msg = document.getElementById('clear-msg') as HTMLHeadingElement;
    const table = document.getElementById('anomalies-table') as HTMLTableElement;
    if (anomalyList.length > 0) {
      msg.hidden = true;
      table.hidden = false;
      this.anomalyList = anomalyList;
      this.groupAnomalies();
      this._render();
    } else {
      msg.hidden = false;
      table.hidden = true;
    }
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
