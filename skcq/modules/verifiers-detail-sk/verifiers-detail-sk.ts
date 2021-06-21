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
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';
import { truncate } from '../../../infra-sk/modules/string';

import { doImpl } from '../skcq';
import {
  GetChangeAttemptsRequest, GetChangeAttemptsResponse, ChangeAttempts, ChangeAttempt, VerifierStatus,
} from '../json';

export class VerifiersDetailSk extends ElementSk {
  // Try renaming this... to show data even when ther eis no data. (then use the same in the other module that is empty most of the time)
  hidden: boolean = true;

  // changeID: number = 0;

  // patchsetID: number = 0;

  changeAttempts: (ChangeAttempts|null) = null;

  constructor() {
    super(VerifiersDetailSk.template);
  }

  private static template = (el: VerifiersDetailSk) => html`
  <div ?hidden=${el.hidden}>
    <h3>SkCQ attempts for <a href="http://skia-review.googlesource.com/c/${el.changeID}/${el.patchsetID}" target=_blank>${el.changeID}/${el.patchsetID}</a></h3>
    ${el.displayAttempts()}
  </div>
`;

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'changeID');
    upgradeProperty(this, 'patchsetID');
    this._render();
  }

  attributeChangedCallback(name: string): void {
    switch (name) {
      case 'change_id':
        this.fetchAttempts();
        break;
      case 'patchset_id':
        this.fetchAttempts();
        break;
      default:
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  static get observedAttributes(): string[] {
    return ['change_id', 'patchset_id'];
  }

  /** @prop change_id The Change ID of the Gerrit change. */
  get changeID(): number {
    return +this.getAttribute('change_id')!;
  }

  /** @prop patchset_id The Patchset ID of the Gerrit change. */
  get patchsetID(): number {
    return +this.getAttribute('patchset_id')!;
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
      <b>Start time</b> ${new Date(attempt!.created * 1000).toLocaleString()}<br/>
      ${this.getCommittedText(attempt!)}
      ${this.displaySubmittableChanges(attempt!)}
      <br/>
      <b>State of Verifiers</b>
      ${this.displayVerifiers(attempt!)}
    </span>
    `);
    /*
        return this.changeAttempts?.attempts.map((attempt) => html`
      <tr>
        <td><a href="http://skia-review.googlesource.com/c/${change?.change_id}/${change?.patchset_id}" target=_blank>${change?.change_id}/${change?.patchset_id}</a></td>
        <td><span title="${change?.change_subject}">${truncate(change?.change_subject as string, 30)}</span></td>
        <td>${change?.change_owner}</td>
        <td><a href="/verifiers_detail/${change?.change_id}/${change?.patchset_id}">Verfiers Details<a></td>
        <td>${diffDate(change?.start_time as number * 1000)}</td>
      </tr>
    `);
    */
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
          ${this.getVerificationDurationText(verifier!.start, verifier!.stop)}
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
    // return html`${lines.join('<br/>')}`;
    return lines.map((line) => html`
      ${escapeAndLinkify(line)}<br/>
    `);
  }

  private getStateClass(state: string): string {
    if (state === 'SUCCESSFUL') {
      return 'success';
    }
    if (state === 'WAITING') {
      return 'waiting';
    }
    if (state === 'ABORTED') {
      return 'aborted';
    }
    return 'failed';
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
    if (attempt.stop) {
      endTime = attempt.stop * 1000;
      additionalInfo = '';
    }
    if (attempt.cq_abandoned) {
      additionalInfo = '(was abandoned)';
    }
    return html`${diffDate(attempt?.created as number * 1000, endTime)} ${additionalInfo}`;
  }

  private getCommittedText(attempt: ChangeAttempt): TemplateResult {
    if (attempt.committed) {
      const d = new Date(attempt.committed * 1000).toLocaleString();
      return html`<b>Patch committed at</b> ${d}<br/>`;
    }
    return html``;
  }

  private fetchAttempts() {
    if (!this.changeID || !this.patchsetID) {
      // Do nothing.
      return;
    }
    const detail: GetChangeAttemptsRequest = {
      change_id: this.changeID,
      patchset_id: this.patchsetID,
    };
    doImpl<GetChangeAttemptsRequest, GetChangeAttemptsResponse>('/_/get_change_attempts', detail, (json: GetChangeAttemptsResponse) => {
      this.changeAttempts = json.change_attempts!;
      this.hidden = this.changeAttempts && this.changeAttempts.attempts!.length === 0;
      this._render();
    });
  }
}

define('verifiers-detail-sk', VerifiersDetailSk);
