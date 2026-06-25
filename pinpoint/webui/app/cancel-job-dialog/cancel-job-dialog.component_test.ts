import 'zone.js';
import 'zone.js/testing';
import '@angular/compiler';
import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { CancelJobDialogComponent } from './cancel-job-dialog.component';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { JobsService } from '../job-list/jobs.service';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('CancelJobDialogComponent', () => {
  before(() => {
    TestBed.resetTestEnvironment();
    TestBed.initTestEnvironment(BrowserTestingModule, platformBrowserTesting());
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  function createComponent(
    jobId: string,
    jobName: string,
    mockDialogRef: any,
    mockJobsService?: Partial<JobsService>
  ): CancelJobDialogComponent {
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: { jobId, jobName } },
        { provide: MatDialogRef, useValue: mockDialogRef },
        { provide: JobsService, useValue: mockJobsService || { cancelJob: async () => {} } },
      ],
    });
    return TestBed.runInInjectionContext(() => new CancelJobDialogComponent());
  }

  const mockJobId = '123456';
  const mockJobName = 'test_job';

  it('should initialize with job data and default state', () => {
    const mockDialogRef = { close: sinon.stub() };
    const component = createComponent(mockJobId, mockJobName, mockDialogRef);

    assert.equal(component.data.jobId, '123456');
    assert.equal(component.data.jobName, 'test_job');
    assert.isFalse(component.loading());
    assert.equal(component.error(), '');
  });

  it('should close dialog with false on dismiss', () => {
    const mockDialogRef = { close: sinon.stub() };
    const component = createComponent(mockJobId, mockJobName, mockDialogRef);

    component.onDismiss();
    assert.isTrue(mockDialogRef.close.calledOnceWithExactly(false));
  });

  it('should call cancelJob and close dialog with true on successful confirm', fakeAsync(() => {
    const mockDialogRef = { close: sinon.stub() };
    const cancelJobSpy = sinon.stub().resolves();
    const component = createComponent(mockJobId, mockJobName, mockDialogRef, {
      cancelJob: cancelJobSpy,
    });

    component.onConfirm();
    assert.isTrue(component.loading());
    tick();

    assert.isTrue(cancelJobSpy.calledOnceWithExactly('123456'));
    assert.isFalse(component.loading());
    assert.equal(component.error(), '');
    assert.isTrue(mockDialogRef.close.calledOnceWithExactly(true));
  }));

  it('should display error message and keep dialog open on cancellation failure', fakeAsync(() => {
    const mockDialogRef = { close: sinon.stub() };
    const cancelJobSpy = sinon.stub().rejects(new Error('Backend error'));
    const component = createComponent(mockJobId, mockJobName, mockDialogRef, {
      cancelJob: cancelJobSpy,
    });

    component.onConfirm();
    assert.isTrue(component.loading());
    tick();

    assert.isTrue(cancelJobSpy.calledOnceWithExactly('123456'));
    assert.isFalse(component.loading());
    assert.equal(component.error(), 'Backend error');
    assert.isFalse(mockDialogRef.close.called);
  }));
});
