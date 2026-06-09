import '@angular/compiler';
import { TestBed } from '@angular/core/testing';
import { HttpErrorResponse } from '@angular/common/http';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { NewJobComponent } from './new-job.component';
import { GatewayService } from '../gateway/gateway.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('NewJobComponent', () => {
  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  function createComponent(mockGateway?: Partial<GatewayService>): NewJobComponent {
    const defaultGateway: Partial<GatewayService> = {
      CreateTryJob: async () => ({ jobId: '12345' }),
    };
    const gateway = { ...defaultGateway, ...mockGateway };
    TestBed.configureTestingModule({
      providers: [{ provide: GatewayService, useValue: gateway }, NewJobComponent],
    });
    return TestBed.runInInjectionContext(() => new NewJobComponent());
  }

  function createValidComponent(mockGateway?: Partial<GatewayService>): NewJobComponent {
    const component = createComponent(mockGateway);
    component.jobForm.get('bot')?.setValue('linux-perf');
    component.jobForm.get('benchmark')?.setValue('speedometer');
    component.jobForm.get('story')?.setValue('Speedometer3');
    component.jobForm.get('baseline.commit')?.setValue('abcd1234');
    return component;
  }

  it('should initialize form with default values', () => {
    const component = createComponent();
    assert.isNotNull(component.jobForm);
    assert.equal(component.jobForm.get('attempts')?.value, 30);
    assert.equal(component.jobForm.get('baseline.commit')?.value, '');
    assert.equal(component.jobForm.get('experiment.commit')?.value, '');
    assert.isFalse(component.jobForm.valid);
  });

  it('should create a valid form', () => {
    const form = createValidComponent().jobForm;
    assert.isTrue(form.valid);
  });

  it('should validate bot', () => {
    const form = createValidComponent().jobForm;
    form.get('bot')?.setValue('');
    assert.isFalse(form.valid);
  });

  it('should validate attempts count', () => {
    const form = createValidComponent().jobForm;
    form.get('attempts')?.setValue(0);
    assert.isFalse(form.valid);

    form.get('attempts')?.setValue(-5);
    assert.isFalse(form.valid);

    form.get('attempts')?.setValue(1);
    assert.isTrue(form.valid);
  });

  it('should validate bug ID', () => {
    const form = createValidComponent().jobForm;
    form.get('bugId')?.setValue('');
    assert.isTrue(form.valid);

    form.get('bugId')?.setValue(0);
    assert.isFalse(form.valid);

    form.get('bugId')?.setValue(-123);
    assert.isFalse(form.valid);
  });

  it('should mark all controls as touched when submitting an invalid form', () => {
    const gateway = {
      CreateTryJob: sinon.stub().resolves({ jobId: 'job_12345' }),
    };
    const component = createComponent(gateway);
    assert.isFalse(component.jobForm.valid);
    assert.isFalse(component.jobForm.touched);

    component.submitJob();

    assert.isTrue(component.jobForm.touched);
    assert.isFalse(component.submitting());
    assert.isTrue(gateway.CreateTryJob.notCalled);
  });

  it('should submit job successfully', async () => {
    const gateway = {
      CreateTryJob: sinon.stub().resolves({ jobId: 'job_12345' }),
    };
    const component = createValidComponent(gateway);

    component.submitJob();

    assert.isTrue(component.submitting());
    // wait for promise resolution
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), 'job_12345');
    assert.equal(component.errorMessage(), '');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  });

  it('should handle submit job failure', async () => {
    const gateway = {
      CreateTryJob: sinon.stub().rejects(new Error('Failed to create')),
    };
    const component = createValidComponent(gateway);

    component.submitJob();

    assert.isTrue(component.submitting());
    // wait for promise resolution
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), '');
    assert.equal(component.errorMessage(), 'Failed to create');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  });

  it('should handle submit job failure with HttpErrorResponse', async () => {
    const errorResponse = new HttpErrorResponse({
      error: { message: 'Invalid bot configuration' },
      status: 400,
      statusText: 'Bad Request',
    });
    const gateway = {
      CreateTryJob: sinon.stub().rejects(errorResponse),
    };
    const component = createValidComponent(gateway);

    component.submitJob();

    assert.isTrue(component.submitting());
    // wait for promise resolution
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), '');
    assert.equal(component.errorMessage(), 'Invalid bot configuration');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  });
});
