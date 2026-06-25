import 'zone.js';
import 'zone.js/testing';
import '@angular/compiler';
import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { JobTableComponent } from './job-table.component';
import { CancelJobDialogComponent } from '../../cancel-job-dialog/cancel-job-dialog.component';

import { JobTableColumnsService, JobTableColumn } from '../job-table-columns.service';
import { GatewayService } from '../../gateway/gateway.service';
import { JobType, JobStatus } from '../../gateway/gateway';
import { JobsService } from '../jobs.service';
import { MatDialog } from '@angular/material/dialog';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('JobTableComponent', () => {
  let stubConsoleError: sinon.SinonStub;

  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  beforeEach(() => {
    stubConsoleError = sinon.stub(console, 'error');
  });

  afterEach(() => {
    stubConsoleError.restore();
    TestBed.resetTestingModule();
  });

  function createComponent(
    mockGateway?: Partial<GatewayService>,
    mockDialog?: any
  ): JobTableComponent {
    const defaultGateway: Partial<GatewayService> = {
      QueryJobList: async () => ({
        jobs: [
          {
            jobId: '123456',
            name: 'test_job',
            benchmark: 'speedometer',
            configuration: 'win-11-perf',
            story: 'default',
            jobType: JobType.JOB_TYPE_TRY,
            user: 'test@google.com',
            created: '2026-05-20T12:00:00Z',
            jobStatus: JobStatus.JOB_STATUS_COMPLETED,
            bugId: 496947065,
          },
        ],
        pagination: { nextCursor: '', prevCursor: '' },
      }),
      GetUserInfo: async () => ({
        email: 'test@google.com',
      }),
    };
    const gateway = { ...defaultGateway, ...mockGateway };
    const dialog = mockDialog || { open: () => {} };
    TestBed.configureTestingModule({
      providers: [
        { provide: GatewayService, useValue: gateway },
        { provide: MatDialog, useValue: dialog },
        JobsService,
      ],
    });
    return TestBed.runInInjectionContext(() => new JobTableComponent());
  }

  it('should load jobs on init successfully', fakeAsync(() => {
    const component = createComponent();
    component.ngOnInit();
    tick();

    assert.isFalse(component.loading());
    assert.isNull(component.error());
    assert.equal(component.jobs().length, 1);
    assert.equal(component.jobs()[0].jobId, '123456');
  }));

  it('should have empty jobs list when no jobs are returned', fakeAsync(() => {
    const component = createComponent({
      QueryJobList: async () => ({
        jobs: [],
        pagination: { nextCursor: '', prevCursor: '' },
      }),
    });
    component.ngOnInit();
    tick();

    assert.isFalse(component.loading());
    assert.isNull(component.error());
    assert.equal(component.jobs().length, 0);
  }));

  it('should handle query failures and set error signal', fakeAsync(() => {
    const testError = new Error('Failed to query');
    const component = createComponent({
      QueryJobList: async () => {
        throw testError;
      },
    });

    component.ngOnInit();
    tick();

    assert.isFalse(component.loading());
    assert.equal(component.error(), 'Failed to query');
    assert.equal(component.jobs().length, 0);
    assert.isTrue(stubConsoleError.calledOnceWithExactly('Failed to load jobs:', testError));
  }));

  it('should reset paginator pageIndex to 0 when showOnlyUserJobs filter changes', fakeAsync(() => {
    const component = createComponent();
    const service = TestBed.inject(JobsService);

    component.paginator = { pageIndex: 3 } as any;

    service.setShowOnlyUserJobs(false);
    tick();

    assert.equal(component.paginator.pageIndex, 0);
  }));

  it('should dynamically retrieve displayedColumns from JobTableColumnsService', () => {
    const component = createComponent();
    const service = TestBed.inject(JobTableColumnsService);

    // Initial: All columns
    assert.equal(component.displayedColumns.length, service.allColumns.length);

    // Select only one column
    service.updateSelection(new Set([JobTableColumn.Name]));
    assert.deepEqual(component.displayedColumns, [JobTableColumn.Name]);

    // Reset selection
    service.updateSelection(new Set(service.allColumns.map((c) => c.id)));
    assert.equal(component.displayedColumns.length, service.allColumns.length);
  });

  it('should call reorderColumns on JobTableColumnsService when columnDrop is invoked', () => {
    const component = createComponent();
    const service = TestBed.inject(JobTableColumnsService);
    const reorderSpy = sinon.spy(service, 'reorderColumns');

    component.columnDrop({ previousIndex: 1, currentIndex: 3 } as any);

    assert.isTrue(reorderSpy.calledOnceWithExactly(1, 3));
  });

  it('should dynamically retrieve column labels from JobTableColumnsService', () => {
    const component = createComponent();
    assert.equal(component.getColumnLabel(JobTableColumn.Name), 'Job Name');
    assert.equal(component.getColumnLabel(JobTableColumn.Configuration), 'Bot');
    assert.equal(component.getColumnLabel(JobTableColumn.JobStatus), 'Status');
    assert.equal(component.getColumnLabel('unknown' as any), 'unknown');
  });

  describe('jobStatusToLabel', () => {
    it('should map job status constants to user friendly names', () => {
      const component = createComponent();
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_QUEUED), '⏳ Queued');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_RUNNING), '🔄 Running');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_COMPLETED), '✅ Completed');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_FAILED), '❌ Failed');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_CANCELLED), '🚫 Cancelled');
      assert.equal(component.jobStatusToLabel('JOB_STATUS_UNKNOWN' as any), '-');
      assert.isTrue(
        stubConsoleError.calledOnceWithExactly('Unknown job status:', 'JOB_STATUS_UNKNOWN')
      );
    });
  });

  describe('getJobTypeLabel', () => {
    it('should map job type constants to friendly names', () => {
      const component = createComponent();
      assert.equal(component.getJobTypeLabel(JobType.JOB_TYPE_BISECT), 'Bisect');
      assert.equal(component.getJobTypeLabel(JobType.JOB_TYPE_TRY), 'Try');
      assert.equal(component.getJobTypeLabel('JOB_TYPE_OTHER' as any), '-');
      assert.isTrue(stubConsoleError.calledOnceWithExactly('Unknown job type:', 'JOB_TYPE_OTHER'));
    });
  });

  describe('sortingDataAccessor', () => {
    it('should return bugId for Bug column', () => {
      const component = createComponent();
      const job = {
        jobId: '1',
        name: 'job_1',
        benchmark: 'b',
        configuration: 'c',
        story: 's',
        jobType: JobType.JOB_TYPE_TRY,
        user: 'u',
        created: '2026-05-20T12:00:00Z',
        jobStatus: JobStatus.JOB_STATUS_COMPLETED,
        bugId: 123,
      };
      assert.equal(component.dataSource.sortingDataAccessor(job, JobTableColumn.Bug), 123);
    });

    it('should return 0 for Bug column if bugId is missing', () => {
      const component = createComponent();
      const job = {
        jobId: '1',
        name: 'job_1',
        benchmark: 'b',
        configuration: 'c',
        story: 's',
        jobType: JobType.JOB_TYPE_TRY,
        user: 'u',
        created: '2026-05-20T12:00:00Z',
        jobStatus: JobStatus.JOB_STATUS_COMPLETED,
      };
      assert.equal(component.dataSource.sortingDataAccessor(job, JobTableColumn.Bug), 0);
    });

    it('should return the property value for other columns', () => {
      const component = createComponent();
      const job = {
        jobId: '1',
        name: 'job_1',
        benchmark: 'b',
        configuration: 'c',
        story: 's',
        jobType: JobType.JOB_TYPE_TRY,
        user: 'u',
        created: '2026-05-20T12:00:00Z',
        jobStatus: JobStatus.JOB_STATUS_COMPLETED,
        bugId: 123,
      };
      assert.equal(component.dataSource.sortingDataAccessor(job, JobTableColumn.Name), 'job_1');
      assert.equal(component.dataSource.sortingDataAccessor(job, JobTableColumn.Benchmark), 'b');
    });
  });

  describe('isCancellable', () => {
    it('should return true only for queued or running jobs created by the current user', fakeAsync(() => {
      const component = createComponent();
      component.ngOnInit();
      tick(); // Let user email resolve to 'test@google.com'

      const createMockJob = (status: JobStatus, user = 'test@google.com') => ({
        jobId: '1',
        name: 'job_1',
        benchmark: 'b',
        configuration: 'c',
        story: 's',
        jobType: JobType.JOB_TYPE_TRY,
        user,
        created: '2026-05-20T12:00:00Z',
        jobStatus: status,
      });

      // Status checks for the owner (test@google.com)
      assert.isTrue(component.isCancellable(createMockJob(JobStatus.JOB_STATUS_QUEUED)));
      assert.isTrue(component.isCancellable(createMockJob(JobStatus.JOB_STATUS_RUNNING)));
      assert.isFalse(component.isCancellable(createMockJob(JobStatus.JOB_STATUS_COMPLETED)));
      assert.isFalse(component.isCancellable(createMockJob(JobStatus.JOB_STATUS_FAILED)));
      assert.isFalse(component.isCancellable(createMockJob(JobStatus.JOB_STATUS_CANCELLED)));

      // Ownership checks (other@google.com)
      assert.isFalse(
        component.isCancellable(createMockJob(JobStatus.JOB_STATUS_QUEUED, 'other@google.com'))
      );
      assert.isFalse(
        component.isCancellable(createMockJob(JobStatus.JOB_STATUS_RUNNING, 'other@google.com'))
      );
    }));
  });

  describe('openCancelDialog', () => {
    it('should open the CancelJobDialogComponent with the job data', fakeAsync(() => {
      const mockDialog = {
        open: sinon.stub(),
      };

      const component = createComponent(undefined, mockDialog);
      component.ngOnInit();
      tick();

      const job = component.jobs()[0];
      assert.isDefined(job, 'Job should be loaded');
      component.openCancelDialog(job);

      assert.isTrue(
        mockDialog.open.calledOnceWithExactly(CancelJobDialogComponent, {
          data: { jobId: job.jobId, jobName: job.name },
        })
      );
    }));
  });
});
