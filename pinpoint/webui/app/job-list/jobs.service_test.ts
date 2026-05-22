import '@angular/compiler';
import { Injector, runInInjectionContext } from '@angular/core';
import { JobsService } from './jobs.service';
import { GatewayService } from '../gateway/gateway.service';
import { JobType } from '../gateway/gateway';
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

  function createService(mockGateway?: Partial<GatewayService>): JobsService {
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
            jobStatus: 'JOB_STATUS_COMPLETED',
          },
        ],
        pagination: { nextCursor: 'next_123', prevCursor: '' },
      }),
    };
    const injector = Injector.create({
      providers: [{ provide: GatewayService, useValue: gateway }, JobsService],
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
    assert.equal(service.pagination()?.nextCursor, 'next_123');
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
});
