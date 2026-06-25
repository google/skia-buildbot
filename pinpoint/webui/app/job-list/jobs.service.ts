import { Injectable, inject, signal } from '@angular/core';
import { GatewayService } from '../gateway/gateway.service';
import { JobSummary, Pagination, JobType, JobStatus } from '../gateway/gateway';
import { SettingsService } from '../settings/settings.service';

@Injectable({
  providedIn: 'root',
})
export class JobsService {
  private gatewayService = inject(GatewayService);

  private settingsService = inject(SettingsService);

  private pageIndex = 0;

  private pageSize = 0;

  private _jobs = signal<JobSummary[]>([]);

  readonly jobs = this._jobs.asReadonly();

  private _showOnlyUserJobs = signal<boolean>(this.settingsService.getShowOnlyUserJobs(true));

  readonly showOnlyUserJobs = this._showOnlyUserJobs.asReadonly();

  private _loading = signal<boolean>(false);

  readonly loading = this._loading.asReadonly();

  private _error = signal<string | null>(null);

  readonly error = this._error.asReadonly();

  private pagination: Pagination | undefined = undefined;

  private _userEmail = signal<string>('');

  readonly userEmail = this._userEmail.asReadonly();

  // Tracks the current active session. Changed whenever the query filters
  // change (e.g. toggling showOnlyUserJobs), which invalidates previous requests.
  private currentSessionId = 0;

  async setShowOnlyUserJobs(showOnlyUserJobs: boolean) {
    if (this._showOnlyUserJobs() === showOnlyUserJobs) {
      return;
    }

    this._showOnlyUserJobs.set(showOnlyUserJobs);
    this.settingsService.setShowOnlyUserJobs(showOnlyUserJobs);
    this._jobs.set([]);
    this.pagination = undefined;
    this.pageIndex = 0;

    this.currentSessionId++;
    this._loading.set(false);

    await this.loadJobs();
  }

  private async updateUserEmail(): Promise<void> {
    if (this.userEmail()) {
      return;
    }

    const response = await this.gatewayService.GetUserInfo({});
    this._userEmail.set(response.email);
  }

  async loadJobs() {
    if (this._loading()) {
      return;
    }

    const sessionId = this.currentSessionId;
    this._loading.set(true);
    this._error.set(null);

    try {
      await this.updateUserEmail();

      while (this.needMoreJobs()) {
        // TODO(b/511988008): Make "user", "configuration", "jobType" fields optional.
        const response = await this.gatewayService.QueryJobList({
          user: this.showOnlyUserJobs() ? this.userEmail() : '',
          configuration: '',
          jobType: JobType.JOB_TYPE_UNSPECIFIED,
          pagination: {
            nextCursor: this.pagination?.nextCursor || '',
            prevCursor: '',
          },
        });

        if (this.currentSessionId !== sessionId) {
          break;
        }

        this.pagination = response.pagination;
        this._jobs.update((existing) => [...existing, ...response.jobs]);
      }
    } catch (err: any) {
      if (this.currentSessionId === sessionId) {
        console.error('Failed to load jobs:', err);
        this._error.set(err?.message || 'An unexpected error occurred.');
      }
    } finally {
      if (this.currentSessionId === sessionId) {
        this._loading.set(false);
      }
    }
  }

  private needMoreJobs(): boolean {
    const lastVisibleJobIndex = (this.pageIndex + 1) * this.pageSize - 1;
    const jobsBuffer = this._jobs().length - lastVisibleJobIndex;
    const jobsToReserve = 100;
    const needJobs = jobsBuffer < jobsToReserve;

    const hasNext = this.pagination === undefined || this.pagination?.hasNext === true;
    return needJobs && hasNext;
  }

  async maybeFetchMore(pageIndex: number, pageSize: number) {
    this.pageIndex = pageIndex;
    this.pageSize = pageSize;
    await this.loadJobs();
  }

  async cancelJob(jobId: string): Promise<void> {
    try {
      await this.gatewayService.CancelJob({
        jobId: jobId,
      });
      this._jobs.update((jobs) =>
        jobs.map((job) =>
          job.jobId === jobId ? { ...job, jobStatus: JobStatus.JOB_STATUS_CANCELLED } : job
        )
      );
    } catch (err: any) {
      console.error('Failed to cancel job:', err);
      this._error.set(err?.message || 'Failed to cancel job.');
      throw err;
    }
  }
}
