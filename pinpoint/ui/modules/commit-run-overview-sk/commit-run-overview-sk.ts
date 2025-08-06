import { LitElement, html, css } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import { MdDialog } from '@material/web/dialog/dialog';
import '@material/web/dialog/dialog.js';
import '@material/web/button/text-button.js';
import '@material/web/icon/icon.js';
import { CommitRunData, JobSchema, TestRun, BuildStatus } from '../../services/api';

@customElement('commit-run-overview-sk')
export class CommitRunOverviewSk extends LitElement {
  @property({ type: Object }) commitRun: CommitRunData | null = null;

  @property({ type: Object }) job: JobSchema | null = null;

  @property({ type: String }) title: string = '';

  @state() private selectedRun: TestRun | null = null;

  @query('md-dialog') private dialog!: MdDialog;

  static styles = css`
    .container {
      border: 1px solid var(--md-sys-color-outline-variant);
      border-radius: 8px;
      padding: 1em;
      background-color: var(--md-sys-color-surface-container-lowest);
    }

    h2,
    h3 {
      margin-top: 0;
    }

    .commit-details {
      color: var(--md-sys-color-on-surface-variant);
      margin-top: -0.5em;
      margin-bottom: 1.5em;
    }

    .build-info,
    .run-info {
      margin-bottom: 1em;
    }

    .info-grid {
      display: grid;
      grid-template-columns: 120px 1fr;
      gap: 0.5em;
    }

    .info-grid dt {
      font-weight: bold;
    }

    .runs-container {
      display: grid;
      grid-template-rows: repeat(2, 20px);
      grid-auto-flow: column;
      gap: 4px;
      overflow-x: auto;
      padding-bottom: 8px; /* for scrollbar */
    }

    .run-box {
      width: 20px;
      height: 20px;
      border-radius: 4px;
      cursor: pointer;
    }

    .run-box.success {
      background-color: var(--pinpoint-success-color);
    }

    .run-box.failure {
      background-color: var(--pinpoint-failure-color);
    }

    .dialog-content {
      min-width: 400px;
    }
  `;

  private showRunDetails(run: TestRun) {
    this.selectedRun = run;
    this.dialog.show();
  }

  private isEmptyValues(run: TestRun, chart?: string): boolean {
    if (!run || !run.Values) {
      return true;
    }
    if (chart) {
      return !run.Values[chart] || run.Values[chart].length === 0;
    }
    // If no chart is specified, check if there are any values for any chart.
    return Object.keys(run.Values).length === 0;
  }

  render() {
    if (!this.job) {
      return html``;
    }

    if (!this.commitRun) {
      return html`<div class="container">
        <h2>${this.title}</h2>
        <p>No commit run data available.</p>
      </div>`;
    }

    const { Build: build, Runs: runs } = this.commitRun;
    const chart = this.job.AdditionalRequestParameters?.chart;

    return html`
      <div class="container">
        <h2>${this.title}</h2>

        ${build && build.Commit && build.Commit.main
          ? html`
              <p class="commit-details">
                <code>${build?.Commit?.main?.git_hash?.substring(0, 12)}</code> &bull;
                ${build.Commit?.main?.author}
              </p>
            `
          : ''}
        ${build
          ? html`
              <div class="build-info">
                <h3>Build</h3>
                <dl class="info-grid">
                  <dt>Builder:</dt>
                  <dd>${build.Device}</dd>
                  <dt>Buildbucket ID:</dt>
                  <dd>
                    <a href="https://ci.chromium.org/b/${build.ID}" target="_blank">${build.ID}</a>
                  </dd>
                  <dt>Status:</dt>
                  <dd>${BuildStatus[build.Status] || 'UNKNOWN'}</dd>
                </dl>
              </div>
            `
          : html`<p>No build information available.</p>`}

        <div class="run-info">
          <h3>Runs (${runs ? runs.length : 0} iterations)</h3>
          <div class="runs-container">
            ${runs && runs.length > 0
              ? runs.map(
                  (run) => html`
                    <div
                      class="run-box ${this.isEmptyValues(run, chart) ? 'failure' : 'success'}"
                      title="Task ID: ${run.TaskID}"
                      @click=${() => this.showRunDetails(run)}></div>
                  `
                )
              : ''}
          </div>
        </div>
      </div>

      <md-dialog>
        <div slot="headline">Run Details</div>
        <div slot="content" class="dialog-content">
          ${this.selectedRun
            ? html`
                <dl class="info-grid">
                  <dt>Task ID:</dt>
                  <dd>
                    <a
                      href="https://chrome-swarming.appspot.com/task?id=${this.selectedRun.TaskID}"
                      target="_blank"
                      >${this.selectedRun.TaskID}</a
                    >
                  </dd>
                  <dt>Bot:</dt>
                  <dd>${this.selectedRun.OSName} (${this.selectedRun.Architecture})</dd>
                  <dt>Status:</dt>
                  <dd>${this.selectedRun.Status}</dd>
                  <dt>Isolate:</dt>
                  <dd>
                    ${this.selectedRun.CAS
                      ? html`
                          <a
                            href="https://cas-viewer.appspot.com/${this.selectedRun.CAS
                              .cas_instance}/blobs/${this.selectedRun.CAS.digest.hash}/${this
                              .selectedRun.CAS.digest.size_bytes}/tree"
                            target="_blank">
                            ${this.selectedRun.CAS.digest.hash.substring(0, 12)}...
                          </a>
                        `
                      : 'N/A'}
                  </dd>
                </dl>
              `
            : ''}
        </div>
        <div slot="actions">
          <md-text-button @click=${() => this.dialog.close()}>Close</md-text-button>
        </div>
      </md-dialog>
    `;
  }
}
