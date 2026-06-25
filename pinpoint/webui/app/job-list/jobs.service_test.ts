import '@angular/compiler';
import { Injector, runInInjectionContext } from '@angular/core';
import { JobsService } from './jobs.service';
import { SettingsService } from '../settings/settings.service';
import { GatewayService } from '../gateway/gateway.service';
import { JobType, JobStatus } from '../gateway/gateway';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('JobsService', () => {
  let stubConsoleError: sinon.SinonStub;

  beforeEach(() => {
    stubConsoleError = sinon.stub(console, 'error');
  });

  afterEach(() => {
    stubConsoleError.restore();
  });

  function createService(
    mockGateway?: Partial<GatewayService>,
    mockSettings?: Partial<SettingsService>
  ): JobsService {
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
          },
        ],
        pagination: { nextCursor: '', prevCursor: '', hasPrev: false, hasNext: false },
      }),
      GetUserInfo: async () => ({
        email: 'test@google.com',
      }),
    };
    const gateway = { ...defaultGateway, ...mockGateway };
    const defaultSettings: Partial<SettingsService> = {
      getShowOnlyUserJobs: (defaultValue: boolean) => defaultValue,
      setShowOnlyUserJobs: () => {},
    };
    const settings = { ...defaultSettings, ...mockSettings };
    const injector = Injector.create({
      providers: [
        { provide: GatewayService, useValue: gateway },
        { provide: SettingsService, useValue: settings },
        JobsService,
      ],
    });
    let service!: JobsService;
    runInInjectionContext(injector, () => {
      service = injector.get(JobsService);
    });
    return service;
  }

  it('should load jobs successfully', async () => {
    const service = createService();
    await service.loadJobs();

    assert.isFalse(service.loading());
    assert.isNull(service.error());
    assert.equal(service.jobs().length, 1);
    assert.equal(service.jobs()[0].jobId, '123456');
    assert.equal((service as any).pagination?.nextCursor, '');
  });

  it('should handle query failures and set error signal', async () => {
    const testError = new Error('Failed to query');
    const service = createService({
      QueryJobList: async () => {
        throw testError;
      },
    });

    await service.loadJobs();

    assert.isFalse(service.loading());
    assert.equal(service.error(), 'Failed to query');
    assert.equal(service.jobs().length, 0);
    assert.isTrue(stubConsoleError.calledOnceWithExactly('Failed to load jobs:', testError));
  });

  it('should load more jobs when maybeFetchMore is called with larger pages', async () => {
    const gateway = {
      QueryJobList: async (req: any) => {
        const cursor = req.pagination?.nextCursor || '';
        if (cursor === '') {
          return {
            jobs: [{ jobId: 'job_1', name: 'job_1', jobType: JobType.JOB_TYPE_TRY } as any],
            pagination: { nextCursor: 'c1', prevCursor: '', hasPrev: false, hasNext: true },
          };
        } else if (cursor === 'c1') {
          return {
            jobs: [{ jobId: 'job_2', name: 'job_2', jobType: JobType.JOB_TYPE_TRY } as any],
            pagination: { nextCursor: 'c2', prevCursor: 'c1', hasPrev: true, hasNext: true },
          };
        } else if (cursor === 'c2') {
          return {
            jobs: [{ jobId: 'job_3', name: 'job_3', jobType: JobType.JOB_TYPE_TRY } as any],
            pagination: { nextCursor: '', prevCursor: 'c2', hasPrev: true, hasNext: false },
          };
        }
        return { jobs: [], pagination: undefined };
      },
    };
    const service = createService(gateway);

    // PageIndex = 0, PageSize = 1: fetches all 3 pages since pre-fetch ahead threshold is 100 jobs
    await service.maybeFetchMore(0, 1);
    assert.equal(service.jobs().length, 3);
    assert.equal(service.jobs()[0].jobId, 'job_1');
    assert.equal(service.jobs()[1].jobId, 'job_2');
    assert.equal(service.jobs()[2].jobId, 'job_3');
    assert.equal((service as any).pagination?.nextCursor, '');

    // PageIndex = 1, PageSize = 1: nextCursor is empty, so jobs list remains 3
    await service.maybeFetchMore(1, 1);
    assert.equal(service.jobs().length, 3);
    assert.equal((service as any).pagination?.nextCursor, '');
  });

  it('should stop pre-fetching when nextCursor is not available', async () => {
    const gateway = {
      QueryJobList: async () => ({
        jobs: [{ jobId: 'job_1', name: 'job_1', jobType: JobType.JOB_TYPE_TRY } as any],
        pagination: { nextCursor: '', prevCursor: '', hasPrev: false, hasNext: false },
      }),
    };
    const service = createService(gateway);

    await service.maybeFetchMore(0, 2);

    // Should remain 1 because nextCursor was empty
    assert.equal(service.jobs().length, 1);
  });

  it('should stop querying when hasNext is false or not present', async () => {
    let queryCount1 = 0;
    const gateway = {
      QueryJobList: async () => {
        queryCount1++;
        return {
          jobs: [{ jobId: 'job_1', name: 'job_1', jobType: JobType.JOB_TYPE_TRY } as any],
          pagination: { nextCursor: 'next', prevCursor: '', hasNext: false },
        };
      },
    };
    const service = createService(gateway);

    await service.maybeFetchMore(0, 1);

    assert.equal(queryCount1, 1);
    assert.equal(service.jobs().length, 1);
    assert.isFalse(service.loading());
  });

  it('should stop querying when the first query returns no jobs', async () => {
    let queryCount = 0;
    const gateway = {
      QueryJobList: async () => {
        queryCount++;
        return {
          jobs: [],
          pagination: { nextCursor: '', prevCursor: '', hasNext: false },
        };
      },
    };
    const service = createService(gateway);

    await service.maybeFetchMore(0, 1);

    assert.equal(queryCount, 1);
    assert.equal(service.jobs().length, 0);
    assert.isFalse(service.loading());
  });

  it('should do nothing if maybeFetchMore is called while loading is true', async () => {
    let queryCount = 0;
    const gateway = {
      QueryJobList: async () => {
        queryCount++;
        return {
          jobs: [{ jobId: 'job_1', name: 'job_1', jobType: JobType.JOB_TYPE_TRY } as any],
          pagination: { nextCursor: '', prevCursor: '', hasPrev: false, hasNext: false },
        };
      },
    };
    const service = createService(gateway);

    // Simulate an ongoing load operation
    (service as any)._loading.set(true);

    // Trigger maybeFetchMore: should do nothing because loading is true
    await service.maybeFetchMore(0, 2);

    assert.equal(queryCount, 0);
  });

  describe('showOnlyUserJobs filtering', () => {
    it('should be true by default', () => {
      const service = createService();
      assert.isTrue(service.showOnlyUserJobs());
    });

    it('should query for user jobs by default using the retrieved email address', async () => {
      let queriedUser = '';
      let getUserInfoCalled = 0;
      const gateway = {
        QueryJobList: async (req: any) => {
          queriedUser = req.user;
          return {
            jobs: [{ jobId: '123' } as any],
            pagination: { nextCursor: '', prevCursor: '', hasPrev: false, hasNext: false },
          };
        },
        GetUserInfo: async () => {
          getUserInfoCalled++;
          return { email: 'somebody@google.com' };
        },
      };
      const service = createService(gateway);

      await service.loadJobs();

      assert.equal(getUserInfoCalled, 1);
      assert.equal(queriedUser, 'somebody@google.com');
    });

    it('should switch to all jobs when showOnlyUserJobs(false) is called', async () => {
      let queriedUser = 'not-empty';
      const gateway = {
        QueryJobList: async (req: any) => {
          queriedUser = req.user;
          return {
            jobs: [{ jobId: '123' } as any],
            pagination: { nextCursor: '', prevCursor: '', hasPrev: false, hasNext: false },
          };
        },
        GetUserInfo: async () => ({ email: 'somebody@google.com' }),
      };
      const service = createService(gateway);

      // Initial load (my jobs by default)
      await service.loadJobs();
      assert.equal(queriedUser, 'somebody@google.com');
      assert.equal(service.jobs().length, 1);

      // Toggle to all jobs
      await service.setShowOnlyUserJobs(false);
      assert.isFalse(service.showOnlyUserJobs());
      // The jobs should be cleared and reloaded with no user filter
      assert.equal(queriedUser, '');
    });

    it('should save preference when showOnlyUserJobs filter changes', async () => {
      const setShowOnlyUserJobsSpy = sinon.spy();
      const service = createService(undefined, {
        setShowOnlyUserJobs: setShowOnlyUserJobsSpy,
      });

      await service.setShowOnlyUserJobs(false);

      assert.isTrue(setShowOnlyUserJobsSpy.calledOnceWithExactly(false));
    });
  });

  describe('cancelJob', () => {
    it('should call gatewayService.CancelJob and update the status of the specified job in the list', async () => {
      const cancelJobSpy = sinon.spy(async (_req: any) => ({}));
      const service = createService({ CancelJob: cancelJobSpy });
      await service.loadJobs();

      assert.equal(service.jobs().length, 1);
      assert.equal(service.jobs()[0].jobId, '123456');
      assert.equal(service.jobs()[0].jobStatus, JobStatus.JOB_STATUS_COMPLETED);

      await service.cancelJob('123456');

      assert.isTrue(
        cancelJobSpy.calledOnceWithExactly({
          jobId: '123456',
        })
      );
      assert.equal(service.jobs()[0].jobStatus, JobStatus.JOB_STATUS_CANCELLED);
    });

    it('should propagate error if gatewayService.CancelJob fails', async () => {
      const errorMsg = 'Failed to cancel job from backend';
      const service = createService({
        CancelJob: async () => {
          throw new Error(errorMsg);
        },
      });
      await service.loadJobs();

      assert.equal(service.jobs().length, 1);
      assert.equal(service.jobs()[0].jobStatus, JobStatus.JOB_STATUS_COMPLETED);

      try {
        await service.cancelJob('123456');
        assert.fail('Should have thrown an error');
      } catch (err: any) {
        assert.equal(err.message, errorMsg);
      }

      // The status should not be updated to Cancelled if the call failed.
      assert.equal(service.jobs()[0].jobStatus, JobStatus.JOB_STATUS_COMPLETED);
      assert.equal(service.error(), errorMsg);
    });
  });
});
