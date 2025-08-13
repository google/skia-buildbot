import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { listJobs, Job, ListJobsOptions, cancelJob } from '../../services/api';
import '../pinpoint-scaffold-sk';
import { stateReflector } from '../../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../../infra-sk/modules/hintable';
import '@material/web/button/filled-button.js';
import '@material/web/button/outlined-button.js';
import '@material/web/dialog/dialog.js';
import '@material/web/textfield/filled-text-field.js';
import { MdDialog } from '@material/web/dialog/dialog.js';
import '../jobs-table-sk';

/**
 * @element pinpoint-landing-page-sk
 *
 * @description The main landing page for the Pinpoint application. It fetches
 * and displays a list of jobs.
 */
@customElement('pinpoint-landing-page-sk')
export class PinpointLandingPageSk extends LitElement {
  static styles = css`
    .pagination-controls {
      display: flex;
      justify-content: flex-end;
      align-items: center;
      gap: 8px;
      margin-top: 16px;
    }

    .empty-message {
      text-align: center;
      padding: 40px;
      color: var(--md-sys-color-on-surface-variant);
    }

    #cancel-dialog p {
      margin-top: 0;
      margin-bottom: 16px;
    }
  `;

  @state() private _pagedJobs: Job[]; // Jobs only for the current page.

  @state() private _loading: boolean;

  @state() private _error: string | null;

  @state() private _sortBy: string;

  @state() private _sortDir: 'asc' | 'desc';

  // Options that control sorting.
  @state() private _listJobsOptions: ListJobsOptions;

  @state() private _currentPage: number;

  @state() private _hasNextPage: boolean;

  @state() private _jobToCancel: Job | null = null;

  private _stateChanged: (() => void) | null = null;

  private readonly _pageSize: number = 20; // We will present 20 jobs per page

  constructor() {
    super();
    this._pagedJobs = [];
    this._loading = true;
    this._error = null;
    this._sortBy = 'created_date';
    this._sortDir = 'desc';
    this._listJobsOptions = {
      searchTerm: '',
      benchmark: '',
      botName: '',
      user: '',
      startDate: '',
      endDate: '',
    };
    this._currentPage = 0;
    this._hasNextPage = false;

    this._stateChanged = stateReflector(
      () => this.getState(),
      (newState) => this.setState(newState as HintableObject)
    );
  }

  private getState(): HintableObject {
    const state: HintableObject = {
      page: this._currentPage,
      sortBy: this._sortBy,
      sortDir: this._sortDir,
      searchTerm: this._listJobsOptions.searchTerm || '',
      benchmark: this._listJobsOptions.benchmark || '',
      botName: this._listJobsOptions.botName || '',
      startDate: this._listJobsOptions.startDate || '',
      endDate: this._listJobsOptions.endDate || '',
      user: this._listJobsOptions.user || '',
    };
    return state;
  }

  private setState(state: HintableObject) {
    this._currentPage = (state.page as number) || 0;
    this._sortBy = (state.sortBy as string) || 'created_date';
    this._sortDir = (state.sortDir as 'asc' | 'desc') || 'desc';

    this._listJobsOptions = {
      searchTerm: (state.searchTerm as string) || '',
      benchmark: (state.benchmark as string) || '',
      botName: (state.botName as string) || '',
      startDate: (state.startDate as string) || '',
      endDate: (state.endDate as string) || '',
      user: (state.user as string) || '',
    };
    this.fetchJobs();
  }

  connectedCallback() {
    super.connectedCallback();
  }

  private async fetchJobs() {
    this._loading = true;
    this._error = null;
    try {
      const options: ListJobsOptions = {
        ...this._listJobsOptions,
        limit: this._pageSize,
        offset: this._currentPage * this._pageSize,
      };
      // The backend may return null for an empty result set.
      const jobs = (await listJobs(options)) || [];
      this._pagedJobs = jobs;
      // If we get back a full page of results, there's likely a next page.
      // Will be fine if there isn't
      this._hasNextPage = jobs.length === this._pageSize;
    } catch (e) {
      this._error = (e as Error).message;
      this._pagedJobs = []; // Clear jobs on error
      this._hasNextPage = false;
    } finally {
      this._loading = false;
    }
  }

  // Handles the 'sort-changed' event from the jobs-table-sk component.
  private onSortChanged(e: CustomEvent<{ sortBy: string; sortDir: 'asc' | 'desc' }>) {
    this._sortBy = e.detail.sortBy;
    this._sortDir = e.detail.sortDir;
    this._stateChanged!();
  }

  // Handles the 'search-changed' event from the pinpoint-scaffold-sk component.
  private onSearchChanged(e: CustomEvent<{ value: string }>) {
    this._listJobsOptions = { ...this._listJobsOptions, searchTerm: e.detail.value };
    this._currentPage = 0; // Reset to first page on new search
    this._stateChanged!();
    this.fetchJobs();
  }

  // Handles the 'filters-changed' event from the pinpoint-scaffold-sk component.
  private onFiltersChanged(
    e: CustomEvent<{
      benchmark: string;
      botName: string;
      user: string;
      startDate: string;
      endDate: string;
    }>
  ) {
    this._listJobsOptions = {
      ...this._listJobsOptions,
      ...e.detail,
    };
    this._currentPage = 0;
    this._stateChanged!();
    this.fetchJobs();
  }

  private onPreviousClicked() {
    if (this._currentPage > 0) {
      this._currentPage -= 1;
      this._stateChanged!();
      this.fetchJobs();
    }
  }

  private onNextClicked() {
    if (this._hasNextPage) {
      this._currentPage += 1;
      this._stateChanged!();
      this.fetchJobs();
    }
  }

  private get cancelDialog() {
    return this.shadowRoot!.querySelector<MdDialog>('#cancel-dialog')!;
  }

  private get cancelReasonField() {
    return this.shadowRoot!.querySelector<HTMLInputElement>('md-filled-text-field')!;
  }

  private handleCancelJobClicked(e: CustomEvent<{ job: Job }>) {
    this._jobToCancel = e.detail.job;
    this.cancelDialog.show();
  }

  private closeCancelDialog() {
    this.cancelDialog.close();
    this.cancelReasonField.value = '';
    this._jobToCancel = null;
  }

  private async submitCancellation() {
    const reason = this.cancelReasonField.value;
    if (!this._jobToCancel || !reason) {
      this._error = 'A reason is required to cancel a job.';
      return;
    }

    this._loading = true;
    try {
      await cancelJob({
        job_id: this._jobToCancel.job_id,
        reason: reason,
      });
      this.closeCancelDialog();
      await this.fetchJobs();
    } catch (e) {
      this._error = `Failed to cancel job: ${(e as Error).message}`;
    } finally {
      this._loading = false;
    }
  }

  render() {
    // Perform client-side sorting of the current page's jobs before rendering.
    const sortedJobs = [...this._pagedJobs].sort((a, b) => {
      const valA = a[this._sortBy as keyof Job] || '';
      const valB = b[this._sortBy as keyof Job] || '';

      if (valA < valB) {
        if (this._sortDir === 'asc') {
          return -1;
        } else {
          return 1;
        }
      }
      if (valA > valB) {
        if (this._sortDir === 'asc') {
          return 1;
        } else {
          return -1;
        }
      }
      return 0;
    });

    const hasPrevious = this._currentPage > 0;
    const hasNext = this._hasNextPage;

    let loadingIndicator = html``;
    if (this._loading) {
      loadingIndicator = html`<p>Loading jobs...</p>`;
    }

    let errorIndicator = html``;
    if (this._error) {
      errorIndicator = html`<p>Error: ${this._error}</p>`;
    }

    let content = html``;
    if (sortedJobs.length > 0) {
      content = html`
        <jobs-table-sk
          .jobs=${sortedJobs}
          .sortBy=${this._sortBy}
          .sortDir=${this._sortDir}
          @sort-changed=${this.onSortChanged}
          @cancel-job-clicked=${this.handleCancelJobClicked}></jobs-table-sk>
        <div class="pagination-controls">
          <md-outlined-button
            ?disabled=${!hasPrevious || this._loading}
            @click=${this.onPreviousClicked}>
            Back
          </md-outlined-button>
          <md-filled-button ?disabled=${!hasNext || this._loading} @click=${this.onNextClicked}>
            Next
          </md-filled-button>
        </div>
      `;
    } else if (!this._loading && !this._error) {
      content = html`<p class="empty-message">No jobs found.</p>`;
    }

    return html`
      <pinpoint-scaffold-sk
        .searchTerm=${this._listJobsOptions.searchTerm}
        .benchmark=${this._listJobsOptions.benchmark}
        .botName=${this._listJobsOptions.botName}
        .user=${this._listJobsOptions.user}
        .startDate=${this._listJobsOptions.startDate}
        .endDate=${this._listJobsOptions.endDate}
        @search-changed=${this.onSearchChanged}
        @filters-changed=${this.onFiltersChanged}>
        ${loadingIndicator} ${errorIndicator} ${content}
        <md-dialog id="cancel-dialog">
          <div slot="headline">Cancel Job</div>
          <form id="cancel-form" slot="content" method="dialog">
            <p>Are you sure you want to cancel job: <b>${this._jobToCancel?.job_name}</b>?</p>
            <md-filled-text-field
              label="Reason"
              required
              style="width: 100%;"></md-filled-text-field>
          </form>
          <div slot="actions">
            <md-outlined-button @click=${this.closeCancelDialog}>Back</md-outlined-button>
            <md-filled-button @click=${this.submitCancellation} ?disabled=${this._loading}>
              Submit
            </md-filled-button>
          </div>
        </md-dialog>
      </pinpoint-scaffold-sk>
    `;
  }
}
