/**
 * @module module/verfiers-detail-sk
 * @description <h2><code>verifiers-detail-sk</code></h2>
 *
 * Displays the details of all verifiers that have run for a
 * change+patchset.
 *
 * @attr {number} change_id - The change ID of the Gerrit change.
 *
 * @attr {number} patchset_id - The patchset ID of the Gerrit change.
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
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';

import { doImpl } from '../skcq';
import {
  GetChangeAttemptsRequest, GetChangeAttemptsResponse, ChangeAttempts, ChangeAttempt, VerifierState,
} from '../json';

const VerifierStateToClassName: Record<VerifierState, string> = {
  SUCCESSFUL: 'success',
  WAITING: 'waiting',
  ABORTED: 'aborted',
  FAILURE: 'failed',
};

export class VerifiersDetailSk extends ElementSk {
  noData: boolean = true;

  updatingData: boolean= true;

  changeAttempts: (ChangeAttempts|null) = null;

  constructor() {
    super(VerifiersDetailSk.template);
  }

  private static template = (el: VerifiersDetailSk) => html`
  <spinner-sk ?active=${el.updatingData}></spinner-sk>
  <div ?hidden=${el.noData}>
    <h3>SkCQ attempts for <a href="http://skia-review.googlesource.com/c/${el.changeID}/${el.patchsetID}" target=_blank>${el.changeID}/${el.patchsetID}</a></h3>
    ${el.displayAttempts()}
  </div>
  ${el.displayNotFoundMsg(el.noData, el.updatingData)}
`;

  connectedCallback(): void {
    super.connectedCallback();
    this.fetchAttempts();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  /** @prop change_id The Change ID of the Gerrit change. */
  get changeID(): number {
    return +this.getAttribute('change_id')!;
  }

  /** @prop patchset_id The Patchset ID of the Gerrit change. */
  get patchsetID(): number {
    return +this.getAttribute('patchset_id')!;
  }

  private displayNotFoundMsg(hidden: boolean, updatingData: boolean): TemplateResult {
    if (hidden && !updatingData) {
      return html`<h3>No data available for <a href="http://skia-review.googlesource.com/c/${this.changeID}/${this.patchsetID}" target=_blank>${this.changeID}/${this.patchsetID}</a></h3>`;
    }
    return html``;
  }

  private displayAttempts(): TemplateResult[] {
    if (!this.changeAttempts || !this.changeAttempts!.attempts) {
      return [];
    }
    return this.changeAttempts!.attempts.reverse().map((attempt, index) => html`
    <span class="attempt-section ${this.getStateClass(attempt!.overall_status)}">
      <table class="attempt-table">
        <tr>
          <th width=33%>
            Attempt #${this.changeAttempts!.attempts!.length - index} ${this.getDryRunText(attempt!)}
          </th>
          <th width=33%>
            ${attempt?.overall_status}
          </th>
          <th width=33%>
            Total duration: ${this.getTotalDurationText(attempt!)}
          </th>
        </tr>
      </table>
      <br/>
      <b>Start time</b> ${new Date(attempt!.start_ts * 1000).toLocaleString()}<br/>
      ${this.getCommittedText(attempt!)}
      ${this.displaySubmittableChanges(attempt!)}
      <br/>
      <b>State of Verifiers</b>
      ${this.displayVerifiers(attempt!)}
    </span>
    `);
  }

  private displayVerifiers(attempt: ChangeAttempt): TemplateResult[] {
    if (!attempt.verifiers_statuses || !attempt.verifiers_statuses.length) {
      return [];
    }
    return attempt.verifiers_statuses.map((verifier) => html`
      <table class="verifier-table ${this.getStateClass(verifier!.state)}">
        <tr>
          <td width=33%>
            ${verifier?.name}
          </td>
          <td width=33%>
          ${verifier?.state}
          </td>
          <td width=33%>
          ${this.getVerificationDurationText(verifier!.start_ts, verifier!.stop_ts)}
          </td>
        </tr>
        <tr>
          <td colspan=3>
          ${this.displayReason(verifier!.reason)}
          </td>
        </tr>
      </table>
    `);
  }

  private displayReason(reason: string): TemplateResult[] {
    const lines = reason.split('\n');
    return lines.map((line) => html`
      ${escapeAndLinkify(line)}<br/>
    `);
  }

  private getStateClass(state: VerifierState): string {
    return VerifierStateToClassName[state];
  }

  private getVerificationDurationText(start: number, stop: number): TemplateResult {
    let endTime = Date.now();
    let additionalInfo = '(still running)';
    if (stop) {
      endTime = stop * 1000;
      additionalInfo = '';
    }
    return html`Total duration: ${diffDate(start * 1000, endTime)} ${additionalInfo}<br/>`;
  }

  private displaySubmittableChanges(attempt: ChangeAttempt): TemplateResult {
    if (!attempt.dry_run && attempt.submittable_changes && attempt.submittable_changes.length > 0) {
      return html`<b>Changes submitted together</b>
      <ul>
        ${attempt.submittable_changes.map((ch) => html`<li><a href="http://skia-review.googlesource.com/c/${ch}" target=_blank>${ch}</a></li>`)}
      </ul>
      `;
    }
    return html``;
  }

  private getDryRunText(attempt: ChangeAttempt): TemplateResult {
    if (attempt.dry_run) {
      return html`(Dry-Run)`;
    }
    return html`(CQ)`;
  }

  private getTotalDurationText(attempt: ChangeAttempt): TemplateResult {
    let endTime = Date.now();
    let additionalInfo = '(still running)';
    if (attempt.stop_ts) {
      endTime = attempt.stop_ts * 1000;
      additionalInfo = '';
    }
    if (attempt.cq_abandoned) {
      additionalInfo = '(was abandoned)';
    }
    return html`${diffDate(attempt?.start_ts as number * 1000, endTime)} ${additionalInfo}`;
  }

  private getCommittedText(attempt: ChangeAttempt): TemplateResult {
    if (attempt.committed_ts) {
      const d = new Date(attempt.committed_ts * 1000).toLocaleString();
      return html`<b>Patch committed at</b> ${d}<br/>`;
    }
    return html``;
  }

  private fetchAttempts() {
    if (!this.changeID || !this.patchsetID) {
      return;
    }
    const detail: GetChangeAttemptsRequest = {
      change_id: this.changeID,
      patchset_id: this.patchsetID,
    };
    doImpl<GetChangeAttemptsRequest, GetChangeAttemptsResponse>('/_/get_change_attempts', detail, (json: GetChangeAttemptsResponse) => {
      this.changeAttempts = json.change_attempts!;
      this.noData = !this.changeAttempts || this.changeAttempts.attempts!.length === 0;
      this.updatingData = false;
      this._render();
    });
  }
}

define('verifiers-detail-sk', VerifiersDetailSk);
