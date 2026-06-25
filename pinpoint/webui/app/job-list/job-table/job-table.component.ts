import { Component, OnInit, AfterViewInit, inject, ViewChild, effect } from '@angular/core';
import { DatePipe } from '@angular/common';
import { MatTableModule, MatTableDataSource } from '@angular/material/table';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatPaginator, MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSort, MatSortModule } from '@angular/material/sort';
import { MatDialog, MatDialogModule } from '@angular/material/dialog';
import { CdkDragDrop, CdkDrag, CdkDropList } from '@angular/cdk/drag-drop';
import { JobTableColumnsService, JobTableColumn } from '../job-table-columns.service';
import { JobSummary, JobType, JobStatus } from '../../gateway/gateway';
import { JobsService } from '../jobs.service';
import { CancelJobDialogComponent } from '../../cancel-job-dialog/cancel-job-dialog.component';

@Component({
  selector: 'app-job-table',
  standalone: true,
  imports: [
    MatTableModule,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatProgressBarModule,
    MatPaginatorModule,
    MatFormFieldModule,
    MatSortModule,
    MatDialogModule,
    DatePipe,
    CdkDrag,
    CdkDropList,
  ],
  templateUrl: './job-table.component.html',
  styleUrls: ['./job-table.component.css'],
})
export class JobTableComponent implements OnInit, AfterViewInit {
  readonly JobTableColumn = JobTableColumn;

  private jobsService = inject(JobsService);

  jobs = this.jobsService.jobs;

  loading = this.jobsService.loading;

  error = this.jobsService.error;

  dataSource = new MatTableDataSource<JobSummary>([]);

  private columnsService = inject(JobTableColumnsService);

  private dialog = inject(MatDialog);

  get displayedColumns(): string[] {
    return this.columnsService.displayedColumns();
  }

  columnDrop(event: CdkDragDrop<string[]>) {
    this.columnsService.reorderColumns(event.previousIndex, event.currentIndex);
  }

  getColumnLabel(columnId: JobTableColumn): string {
    return this.columnsService.allColumns.find((c) => c.id === columnId)?.label || columnId;
  }

  @ViewChild(MatPaginator) paginator!: MatPaginator;

  @ViewChild(MatSort) sort!: MatSort;

  constructor() {
    this.dataSource.sortingDataAccessor = (item: JobSummary, property: string): string | number => {
      if (property === JobTableColumn.Bug) {
        return item.bugId ?? 0;
      }
      const value = item[property as keyof JobSummary];
      return (value ?? '') as string | number;
    };

    // Update jobs table data source when fetched jobs are updated.
    effect(() => {
      if (this.dataSource.data !== this.jobs()) {
        this.dataSource.data = this.jobs();
      }
    });

    // Reset page index after switching between "all jobs" and "my jobs".
    effect(() => {
      this.jobsService.showOnlyUserJobs();
      if (this.paginator) {
        this.paginator.pageIndex = 0;
      }
    });
  }

  ngOnInit() {
    this.jobsService.loadJobs();
  }

  ngAfterViewInit() {
    this.dataSource.paginator = this.paginator;
    this.dataSource.sort = this.sort;
    this.jobsService.maybeFetchMore(this.paginator.pageIndex, this.paginator.pageSize);
  }

  onPageChange(event: PageEvent) {
    this.jobsService.maybeFetchMore(event.pageIndex, event.pageSize);
  }

  jobStatusToLabel(status: JobStatus): string {
    switch (status) {
      case JobStatus.JOB_STATUS_QUEUED:
        return '⏳ Queued';
      case JobStatus.JOB_STATUS_RUNNING:
        return '🔄 Running';
      case JobStatus.JOB_STATUS_COMPLETED:
        return '✅ Completed';
      case JobStatus.JOB_STATUS_FAILED:
        return '❌ Failed';
      case JobStatus.JOB_STATUS_CANCELLED:
        return '🚫 Cancelled';
      case JobStatus.JOB_STATUS_UNSPECIFIED:
      case JobStatus.UNRECOGNIZED:
      default:
        console.error('Unknown job status:', status);
        return '-';
    }
  }

  getJobTypeLabel(jobType: JobType): string {
    switch (jobType) {
      case JobType.JOB_TYPE_BISECT:
        return 'Bisect';
      case JobType.JOB_TYPE_TRY:
        return 'Try';
      case JobType.JOB_TYPE_UNSPECIFIED:
      case JobType.UNRECOGNIZED:
      default:
        console.error('Unknown job type:', jobType);
        return '-';
    }
  }

  isCancellable(job: JobSummary): boolean {
    const cancellableStatus = [JobStatus.JOB_STATUS_QUEUED, JobStatus.JOB_STATUS_RUNNING].includes(
      job.jobStatus
    );
    const isOwner = job.user === this.jobsService.userEmail();
    return isOwner && cancellableStatus;
  }

  openCancelDialog(job: JobSummary) {
    this.dialog.open(CancelJobDialogComponent, {
      data: { jobId: job.jobId, jobName: job.name },
    });
  }
}
