import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { listJobs, Job, ListJobsOptions } from '../../services/api';
import '../pinpoint-scaffold-sk';
import '@material/web/button/filled-button.js';
import '@material/web/button/outlined-button.js';
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

  private readonly _pageSize: number = 20; // We will present 20 jobs per page

  constructor() {
    super();
    this._pagedJobs = [];
    this._loading = true;
    this._error = null;
    this._sortBy = 'created_date';
    this._sortDir = 'desc';
    this._listJobsOptions = { searchTerm: '' };
    this._currentPage = 0;
    this._hasNextPage = false;
  }

  connectedCallback() {
    super.connectedCallback();
    this.fetchJobs();
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
  }

  // Handles the 'search-changed' event from the pinpoint-scaffold-sk component.
  private onSearchChanged(e: CustomEvent<{ value: string }>) {
    this._listJobsOptions = { ...this._listJobsOptions, searchTerm: e.detail.value };
    this._currentPage = 0; // Reset to first page on new search
    this.fetchJobs();
  }

  private onPreviousClicked() {
    if (this._currentPage > 0) {
      this._currentPage -= 1;
      this.fetchJobs();
    }
  }

  private onNextClicked() {
    if (this._hasNextPage) {
      this._currentPage += 1;
      this.fetchJobs();
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
          @sort-changed=${this.onSortChanged}></jobs-table-sk>
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
      <pinpoint-scaffold-sk @search-changed=${this.onSearchChanged}>
        ${loadingIndicator} ${errorIndicator} ${content}
      </pinpoint-scaffold-sk>
    `;
  }
}
