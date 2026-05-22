import '@angular/compiler';
import { Injector, runInInjectionContext } from '@angular/core';
import { JobTableComponent, JobTableColumn } from './job-table.component';
import { GatewayService } from '../../gateway/gateway.service';
import { JobType, JobStatus } from '../../gateway/gateway';
import { JobsService } from '../jobs.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('JobTableComponent', () => {
  let stubConsoleError: sinon.SinonStub;

  beforeEach(() => {
    stubConsoleError = sinon.stub(console, 'error');
  });

  afterEach(() => {
    stubConsoleError.restore();
  });

  function createComponent(mockGateway?: Partial<GatewayService>): JobTableComponent {
    const gateway = mockGateway || {
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
          },
        ],
        pagination: { nextCursor: 'next_123', prevCursor: '' },
      }),
    };
    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: gateway }, JobsService],
    });
    let component!: JobTableComponent;
    runInInjectionContext(injector, () => {
      component = new JobTableComponent();
    });
    return component;
  }

  it('should load jobs on init successfully', async () => {
    const component = createComponent();
    await component.ngOnInit();

    assert.isFalse(component.loading());
    assert.isNull(component.error());
    assert.equal(component.jobs().length, 1);
    assert.equal(component.jobs()[0].jobId, '123456');
    assert.equal(component.pagination()?.nextCursor, 'next_123');
  });

  it('should handle query failures and set error signal', async () => {
    const testError = new Error('Failed to query');
    const component = createComponent({
      QueryJobList: async () => {
        throw testError;
      },
    });

    await component.ngOnInit();

    assert.isFalse(component.loading());
    assert.equal(component.error(), 'Failed to query');
    assert.equal(component.jobs().length, 0);
    assert.isTrue(stubConsoleError.calledOnceWithExactly('Failed to load jobs:', testError));
  });

  it('should initialize displayedColumns correctly', () => {
    const component = createComponent();
    assert.deepEqual(component.displayedColumns, [
      JobTableColumn.Name,
      JobTableColumn.Benchmark,
      JobTableColumn.Configuration,
      JobTableColumn.Story,
      JobTableColumn.JobType,
      JobTableColumn.User,
      JobTableColumn.Created,
      JobTableColumn.JobStatus,
    ]);
  });

  describe('jobStatusToLabel', () => {
    it('should map job status constants to user friendly names', () => {
      const component = createComponent();
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_QUEUED), 'Queued');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_RUNNING), 'Running');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_COMPLETED), 'Completed');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_FAILED), 'Failed');
      assert.equal(component.jobStatusToLabel(JobStatus.JOB_STATUS_CANCELLED), 'Cancelled');
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
});
