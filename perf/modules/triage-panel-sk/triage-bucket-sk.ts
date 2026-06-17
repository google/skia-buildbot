/**
 * @module modules/triage-panel-sk/triage-bucket-sk
 * @description <h2><code>triage-bucket-sk</code></h2>
 *
 * Represents a single custom triage bucket card. Implements a smart state machine (NEW -> APPLIED -> DIRTY)
 * to handle initial triage actions, display status badges, and track pending changes for re-application.
 */
import { html, LitElement } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import { Anomaly } from '../json';
import { NewBugDialogSk } from '../new-bug-dialog-sk/new-bug-dialog-sk';
import { ExistingBugDialogSk } from '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { makeEditAnomalyRequest } from '../anomalies-table-sk/triage-api';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { TriageBucket, updateBucketDirtyState } from './buckets-controller';

import '../new-bug-dialog-sk/new-bug-dialog-sk';
import '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import '../../../elements-sk/modules/icons/clear-icon-sk';
import '../../../elements-sk/modules/icons/delete-icon-sk';

@customElement('triage-bucket-sk')
export class TriageBucketSk extends LitElement {
  @property({ attribute: false })
  bucket!: TriageBucket;

  @property({ attribute: false })
  anomalies: Anomaly[] = [];

  @property({ attribute: false })
  traceNames: string[] = [];

  @query('new-bug-dialog-sk')
  newBugDialog!: NewBugDialogSk;

  @query('existing-bug-dialog-sk')
  existingBugDialog!: ExistingBugDialogSk;

  createRenderRoot() {
    return this;
  }

  private emitBucketUpdated(updatedBucket: TriageBucket) {
    this.dispatchEvent(
      new CustomEvent('bucket-updated', {
        detail: { bucket: updatedBucket },
        bubbles: true,
        composed: true,
      })
    );
  }

  render() {
    const b = this.bucket;
    return html`
      <div class="bucket-card">
        <div class="bucket-header">
          <div class="title-and-controls">
            <h3>${b.name} (${b.anomalies.length})</h3>
            ${b.anomalies.length > 0
              ? html`
                  <button
                    class="icon-btn"
                    title="Clean (Empty staged anomalies)"
                    @click=${this.onClean}>
                    <clear-icon-sk></clear-icon-sk>
                  </button>
                `
              : html`
                  <button class="icon-btn" title="Delete bucket entirely" @click=${this.onDelete}>
                    <delete-icon-sk></delete-icon-sk>
                  </button>
                `}
          </div>

          <div class="actions-group">
            ${b.triageState === 'NEW'
              ? html`
                  <button
                    class="primary ignore-btn"
                    ?disabled=${b.anomalies.length === 0}
                    @click=${this.onIgnore}>
                    Ignore
                  </button>
                  <button
                    class="primary new-bug-btn"
                    ?disabled=${b.anomalies.length === 0}
                    @click=${this.onNewBug}>
                    New Bug
                  </button>
                  <button
                    class="primary existing-bug-btn"
                    ?disabled=${b.anomalies.length === 0}
                    @click=${this.onExistingBug}>
                    Existing Bug
                  </button>
                `
              : html`
                  <span class="status-badge">
                    ${b.actionType === 'IGNORE'
                      ? 'Status: Ignored'
                      : html`Bug
                          <a href="https://issues.chromium.org/issues/${b.bugId}" target="_blank"
                            >#${b.bugId}</a
                          >`}
                  </span>
                  <button
                    class="primary apply-btn"
                    ?disabled=${b.triageState !== 'DIRTY'}
                    @click=${this.onApply}>
                    Apply ${b.triageState === 'DIRTY' ? '(Pending)' : ''}
                  </button>
                `}
          </div>
        </div>

        <textarea
          class="bucket-textarea"
          placeholder="No anomalies staged. Click [${b.name}] on table rows, or paste anomaly IDs here."
          .value=${b.anomalies
            .map((a) => `[${a.start_revision}-${a.end_revision}]:${a.test_path}:${a.id}`)
            .join('\n')}
          @input=${this.onTextareaInput}>
        </textarea>

        <new-bug-dialog-sk
          .anomalies=${b.anomalies}
          .traceNames=${this.traceNames}
          @anomaly-changed=${this.onDialogAnomalyChanged}>
        </new-bug-dialog-sk>
        <existing-bug-dialog-sk
          .anomalies=${b.anomalies}
          .traceNames=${this.traceNames}
          @anomaly-changed=${this.onDialogAnomalyChanged}>
        </existing-bug-dialog-sk>
      </div>
    `;
  }

  private onClean() {
    const updated = updateBucketDirtyState({ ...this.bucket, anomalies: [] });
    this.emitBucketUpdated(updated);
  }

  private onDelete() {
    this.dispatchEvent(
      new CustomEvent('bucket-deleted', {
        detail: { name: this.bucket.name },
        bubbles: true,
        composed: true,
      })
    );
  }

  private onTextareaInput(e: Event) {
    const text = (e.target as HTMLTextAreaElement).value;
    const pastedIds = text
      .split(/[\n,]+/)
      .map((s) => s.trim().split(':').pop()!)
      .filter(Boolean);

    const matching = this.anomalies.filter((a) => pastedIds.includes(a.id));
    const updated = updateBucketDirtyState({ ...this.bucket, anomalies: matching });
    this.emitBucketUpdated(updated);
  }

  private async onIgnore() {
    try {
      await makeEditAnomalyRequest(this.bucket.anomalies, this.traceNames, 'IGNORE');
      const updated: TriageBucket = {
        ...this.bucket,
        actionType: 'IGNORE',
        triageState: 'APPLIED',
        lastAppliedIds: this.bucket.anomalies.map((a) => a.id),
      };
      this.emitBucketUpdated(updated);
      this.fireAnomalyChanged('IGNORE');
    } catch (_e) {
      // errorMessage handled by makeEditAnomalyRequest
    }
  }

  private onNewBug() {
    this.newBugDialog.setAnomalies(this.bucket.anomalies, this.traceNames);
    this.newBugDialog.fileNewBug();
  }

  private onExistingBug() {
    this.existingBugDialog.setAnomalies(this.bucket.anomalies, this.traceNames);
    this.existingBugDialog.fetch_associated_bugs();
    this.existingBugDialog.open();
  }

  private onDialogAnomalyChanged(e: CustomEvent) {
    const bugId = e.detail.bugId as number;
    if (bugId !== undefined) {
      const updated: TriageBucket = {
        ...this.bucket,
        actionType: 'EXISTING_BUG',
        bugId: bugId,
        triageState: 'APPLIED',
        lastAppliedIds: this.bucket.anomalies.map((a) => a.id),
      };
      this.emitBucketUpdated(updated);
    }
  }

  private async onApply() {
    const lastApplied = new Set(this.bucket.lastAppliedIds || []);
    const unapplied = this.bucket.anomalies.filter((a) => !lastApplied.has(a.id));
    if (unapplied.length === 0) return;

    try {
      if (this.bucket.actionType === 'IGNORE') {
        await makeEditAnomalyRequest(unapplied, this.traceNames, 'IGNORE');
        this.fireAnomalyChanged('IGNORE');
      } else if (this.bucket.actionType === 'EXISTING_BUG' && this.bucket.bugId) {
        await this.associateAlerts(this.bucket.bugId, unapplied, this.traceNames);
        this.fireAnomalyChanged('ASSOCIATE');
      }

      const updated: TriageBucket = {
        ...this.bucket,
        triageState: 'APPLIED',
        lastAppliedIds: this.bucket.anomalies.map((a) => a.id),
      };
      this.emitBucketUpdated(updated);
    } catch (_e) {
      // errorMessage handled by utility
    }
  }

  private async associateAlerts(bugId: number, anomalies: Anomaly[], traceNames: string[]) {
    const requestBody = {
      bug_id: bugId,
      keys: anomalies.map((a) => a.id),
      trace_names: traceNames,
    };
    const res = await fetch('/_/triage/associate_alerts', {
      method: 'POST',
      body: JSON.stringify(requestBody),
      headers: { 'Content-Type': 'application/json' },
    });
    await jsonOrThrow(res);
    anomalies.forEach((a) => (a.bug_id = bugId));
  }

  private fireAnomalyChanged(action: string) {
    this.dispatchEvent(
      new CustomEvent('anomaly-changed', {
        bubbles: true,
        composed: true,
        detail: {
          traceNames: this.traceNames,
          editAction: action,
          anomalies: this.bucket.anomalies,
        },
      })
    );
  }
}
