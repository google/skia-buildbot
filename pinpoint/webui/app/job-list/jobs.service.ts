import { Injectable, inject, signal } from '@angular/core';
import { GatewayService } from '../gateway/gateway.service';
import { JobSummary, Pagination, JobType, GetUserInfoResponse } from '../gateway/gateway';

@Injectable({
  providedIn: 'root',
})
export class JobsService {
  private gatewayService = inject(GatewayService);

  private pageIndex = 0;

  private pageSize = 0;

  private _jobs = signal<JobSummary[]>([]);

  readonly jobs = this._jobs.asReadonly();

  private _showOnlyUserJobs = signal<boolean>(true);

  readonly showOnlyUserJobs = this._showOnlyUserJobs.asReadonly();

  private _loading = signal<boolean>(false);

  readonly loading = this._loading.asReadonly();

  private _error = signal<string | null>(null);

  readonly error = this._error.asReadonly();

  private pagination: Pagination | undefined = undefined;

  private userEmail = '';

  async setShowOnlyUserJobs(showOnlyUserJobs: boolean) {
    if (this._showOnlyUserJobs() === showOnlyUserJobs) {
      return;
    }
    this._showOnlyUserJobs.set(showOnlyUserJobs);
    this._jobs.set([]);
    this.pagination = undefined;
    this.pageIndex = 0;
    await this.loadJobs();
  }

  private async updateUserEmail(): Promise<void> {
    if (this.userEmail) {
      return;
    }

    const userInfoFuture = this.gatewayService.GetUserInfo({});
    if (this._showOnlyUserJobs()) {
      const response = await userInfoFuture;
      this.userEmail = response.email;
    } else {
      userInfoFuture.then((response: GetUserInfoResponse) => {
        this.userEmail = response.email;
      });
    }
  }

  async loadJobs() {
    if (this._loading()) {
      return;
    }

    this._loading.set(true);
    this._error.set(null);

    try {
      await this.updateUserEmail();

      while (this.needMoreJobs()) {
        // TODO(b/511988008): Make "user", "configuration", "jobType" fields optional.
        const response = await this.gatewayService.QueryJobList({
          user: this._showOnlyUserJobs() ? this.userEmail : '',
          configuration: '',
          jobType: JobType.JOB_TYPE_UNSPECIFIED,
          pagination: {
            nextCursor: this.pagination?.nextCursor || '',
            prevCursor: '',
          },
        });
        this.pagination = response.pagination;

        if (this._jobs().length === 0) {
          this._jobs.set(response.jobs);
        } else {
          this._jobs.update((existing) => [...existing, ...response.jobs]);
        }
      }
    } catch (err: any) {
      console.error('Failed to load jobs:', err);
      this._error.set(err?.message || 'An unexpected error occurred.');
    } finally {
      this._loading.set(false);
    }
  }

  private needMoreJobs(): boolean {
    const lastVisibleJobIndex = (this.pageIndex + 1) * this.pageSize - 1;
    const jobsBuffer = this._jobs().length - lastVisibleJobIndex;
    const jobsToReserve = 100;
    const needJobs = jobsBuffer < jobsToReserve;

    const hasNext = this._jobs().length === 0 || this.pagination?.hasNext === true;
    return needJobs && hasNext;
  }

  async maybeFetchMore(pageIndex: number, pageSize: number) {
    this.pageIndex = pageIndex;
    this.pageSize = pageSize;
    await this.loadJobs();
  }
}
