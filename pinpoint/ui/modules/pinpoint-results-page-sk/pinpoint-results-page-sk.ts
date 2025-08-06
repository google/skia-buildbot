import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { JobSchema, getJob } from '../../services/api';
import '@material/web/button/outlined-button.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/icon/icon.js';
import '@material/web/dialog/dialog.js';
import '../job-overview-sk';
import { JobOverviewSk } from '../job-overview-sk/job-overview-sk';
import '../commit-run-overview-sk';
import '../../../../elements-sk/modules/icons/home-icon-sk';

@customElement('pinpoint-results-page-sk')
export class ResultsPageSk extends LitElement {
  @state() private job: JobSchema | null = null;

  @state() private error: string = '';

  @state() private loading: boolean = true;

  static styles = css`
    .container {
      padding: 1em;
    }
    .header {
      display: flex;
      align-items: center;
      gap: 1em;
    }
    .title {
      font-size: 1.5em;
      font-weight: bold;
    }
    .subtitle {
      color: var(--md-sys-color-on-surface-variant);
      margin-top: 0.5em;
      padding-left: 4em;
      display: flex;
      gap: 1.5em;
      font-size: 0.9em;
    }
    .top-right {
      margin-left: auto;
    }
    .commit-runs-section {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 1em;
      margin-top: 1em;
    }
  `;

  connectedCallback() {
    super.connectedCallback();
    this.fetchJob();
  }

  private async fetchJob() {
    const pathParts = window.location.pathname.split('/');
    const jobId = pathParts[pathParts.length - 1];
    if (!jobId) {
      this.error = 'No Job ID found in URL.';
      this.loading = false;
      return;
    }

    try {
      this.job = await getJob(jobId);
    } catch (e) {
      this.error = `Failed to fetch job: ${(e as Error).message}`;
    } finally {
      this.loading = false;
    }
  }

  private openArgumentsDialog() {
    const dialog = this.shadowRoot?.querySelector<JobOverviewSk>('job-overview-sk');
    dialog?.show();
  }

  private formatDate(dateStr: string): string {
    if (!dateStr) return '';
    return new Intl.DateTimeFormat(navigator.language, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date(dateStr));
  }

  render() {
    if (this.loading) return html`<p>Loading...</p>`;
    if (this.error) return html`<p>Error: ${this.error}</p>`;
    if (!this.job) return html`<p>No job data.</p>`;

    const duration = this.job.AdditionalRequestParameters?.duration
      ? `${this.job.AdditionalRequestParameters.duration} minutes`
      : 'N/A';

    const commitRuns = this.job.AdditionalRequestParameters?.commit_runs;

    return html`
      <div class="container">
        <div class="header">
          <a href="/"><home-icon-sk></home-icon-sk></a>
          <div class="title">Try Job: ${this.job.JobName}</div>
          <div class="top-right">
            <md-outlined-button @click=${this.openArgumentsDialog}
              >View Arguments</md-outlined-button
            >
          </div>
        </div>
        <div class="subtitle">
          <span>User: ${this.job.SubmittedBy}</span>
          <span>Created On: ${this.formatDate(this.job.CreatedDate)}</span>
          <span>Duration: ${duration}</span>
        </div>
        <div class="commit-runs-section">
          <commit-run-overview-sk
            title="Base Commit"
            .job=${this.job}
            .commitRun=${commitRuns?.left || null}></commit-run-overview-sk>
          <commit-run-overview-sk
            title="Experimental Commit"
            .job=${this.job}
            .commitRun=${commitRuns?.right || null}></commit-run-overview-sk>
        </div>
      </div>
      <job-overview-sk .job=${this.job}></job-overview-sk>
    `;
  }
}
