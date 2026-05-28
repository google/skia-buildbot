import { Injectable, inject, signal } from '@angular/core';
import { GatewayService } from '../gateway/gateway.service';
import { JobSummary, Pagination, JobType } from '../gateway/gateway';

@Injectable({
  providedIn: 'root',
})
export class JobsService {
  private gatewayService = inject(GatewayService);

  private pageIndex = 0;

  private pageSize = 0;

  private _jobs = signal<JobSummary[]>([]);

  readonly jobs = this._jobs.asReadonly();

  private _loading = signal<boolean>(false);

  readonly loading = this._loading.asReadonly();

  private _error = signal<string | null>(null);

  readonly error = this._error.asReadonly();

  private pagination: Pagination | undefined = undefined;

  async loadJobs() {
    if (this._loading()) {
      return;
    }

    this._loading.set(true);
    this._error.set(null);

    try {
      while (this.needMoreJobs()) {
        // TODO(b/511988008): Make "user", "configuration", "jobType" fields optional.
        const response = await this.gatewayService.QueryJobList({
          user: '',
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
