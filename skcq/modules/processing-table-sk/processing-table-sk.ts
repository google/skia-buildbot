/**
 * @module module/skcq
 * @description <h2><code>skcq</code></h2>
 *
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { upgradeProperty } from 'elements-sk/upgradeProperty';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/spinner-sk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import { truncate } from '../../../infra-sk/modules/string';

import { doImpl } from '../skcq';
import { GetCurrentChangesRequest, GetCurrentChangesResponse, CQRecord } from '../json';

export class ProcessingTableSk extends ElementSk {
  tableHidden: boolean = true;

  changes: (CQRecord|null)[] = [];

  constructor() {
    super(ProcessingTableSk.template);
  }

  private static template = (el: ProcessingTableSk) => html`
  <div ?hidden=${el.tableHidden}>
    <h3>${el.getChangesType()} changes being processed</h3>
    <table class="current-changes-table">
      <tr class="headers">
        <th>CL</th>
        <th>Subject</th>
        <th>Owner</th>
        <th>Repo/Branch</th>
        <th>Verfiers Details</th>
        <th>Elapsed</th>
      </tr>
      ${el.displayChanges()}
    </table>
  </div>
  ${el.displayNoChangesMsg(el.tableHidden)}
`;

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'isDryRun');
    this._render();
  }

  attributeChangedCallback(name: string): void {
    switch (name) {
      case 'is_dry_run':
        this.fetchTasks();
        break;
      default:
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  static get observedAttributes(): string[] {
    return ['is_dry_run'];
  }

  /** @prop is_dry_run {Bool} User tasks should be filtered by. */
  get isDryRun(): boolean {
    return this.getAttribute('is_dry_run')! === 'true';
  }

  private displayNoChangesMsg(hidden: boolean): TemplateResult {
    if (hidden) {
      return html`<h3>No ${this.getChangesType()} changes in queue</h3>`;
    }
    return html``;
  }

  private displayChanges(): TemplateResult[]|TemplateResult {
    return this.changes.map((change) => html`
      <tr>
        <td><a href="http://skia-review.googlesource.com/c/${change?.change_id}/${change?.patchset_id}" target=_blank>${change?.change_id}/${change?.patchset_id}</a></td>
        <td><span title="${change?.change_subject}">${truncate(change?.change_subject as string, 30)}</span></td>
        <td>${change?.change_owner}</td>
        <td><a href="https://skia-review.googlesource.com/q/project:${change?.repo}+branch:${change?.branch}+status:open" target=_branch>${change?.repo}/${change?.branch}</a></td>
        <td><a href="/verifiers_detail/${change?.change_id}/${change?.patchset_id}">Verfiers Details<a></td>
        <td>${diffDate(change?.start_time as number * 1000)}</td>
      </tr>
    `);
  }

  private getChangesType(): string {
    if (this.isDryRun) {
      return 'Dry-run';
    }
    return 'CQ';
  }

  private fetchTasks() {
    const detail: GetCurrentChangesRequest = {
      is_dry_run: this.isDryRun,
    };
    doImpl<GetCurrentChangesRequest, GetCurrentChangesResponse>('/_/get_current_changes', detail, (json: GetCurrentChangesResponse) => {
      this.changes = json.changes!;
      this.tableHidden = this.changes.length === 0;
      this._render();
    });
  }
}

define('processing-table-sk', ProcessingTableSk);
