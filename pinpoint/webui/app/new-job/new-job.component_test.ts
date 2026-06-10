import 'zone.js';
import 'zone.js/testing';
import '@angular/compiler';
import { TestBed, fakeAsync, tick } from '@angular/core/testing';
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
      ListBotConfigurations: async () => ({ configurations: [] }),
      ListBenchmarks: async () => ({ benchmarks: [] }),
    };
    const gateway = { ...defaultGateway, ...mockGateway };
    TestBed.configureTestingModule({
      providers: [{ provide: GatewayService, useValue: gateway }, NewJobComponent],
    });
    return TestBed.runInInjectionContext(() => new NewJobComponent());
  }

  function createValidComponent(mockGateway?: Partial<GatewayService>): NewJobComponent {
    const component = createComponent(mockGateway);
    component.bots.set(['linux-perf']);
    component.jobForm.get('bot')?.setValue('linux-perf');
    component.benchmarks.set(['speedometer']);
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

  it('should validate bot autocomplete values', () => {
    const component = createComponent();
    component.bots.set(['linux-perf', 'win-perf']);

    component.jobForm.get('bot')?.setValue('linux-perf');
    assert.isTrue(component.jobForm.get('bot')?.valid);

    component.jobForm.get('bot')?.setValue('unknown-bot');
    assert.isTrue(component.jobForm.get('bot')?.hasError('invalidAutocomplete'));
  });

  it('should validate benchmark autocomplete values', () => {
    const component = createComponent();
    component.benchmarks.set(['speedometer3', 'jetstream2']);

    component.jobForm.get('benchmark')?.setValue('speedometer3');
    assert.isTrue(component.jobForm.get('benchmark')?.valid);

    component.jobForm.get('benchmark')?.setValue('unknown-benchmark');
    assert.isTrue(component.jobForm.get('benchmark')?.hasError('invalidAutocomplete'));
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

  it('should submit job successfully', fakeAsync(() => {
    const gateway = {
      CreateTryJob: sinon.stub().resolves({ jobId: 'job_12345' }),
    };
    const component = createValidComponent(gateway);

    component.submitJob();

    assert.isTrue(component.submitting());
    tick();

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), 'job_12345');
    assert.equal(component.errorMessage(), '');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  }));

  it('should handle submit job failure', fakeAsync(() => {
    const gateway = {
      CreateTryJob: sinon.stub().rejects(new Error('Failed to create')),
    };
    const component = createValidComponent(gateway);

    component.submitJob();

    assert.isTrue(component.submitting());
    tick();

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), '');
    assert.equal(component.errorMessage(), 'Failed to create');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  }));

  it('should handle submit job failure with HttpErrorResponse', fakeAsync(() => {
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
    tick();

    assert.isFalse(component.submitting());
    assert.equal(component.createdJobId(), '');
    assert.equal(component.errorMessage(), 'Invalid bot configuration');
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  }));

  it('should fetch bots on initialization', fakeAsync(() => {
    const gateway = {
      ListBotConfigurations: sinon.stub().resolves({ configurations: ['bot1', 'bot2'] }),
    };
    const component = createComponent(gateway);

    component.ngOnInit();
    tick();

    assert.deepEqual(component.bots(), ['bot1', 'bot2']);
    assert.deepEqual(component.filteredBots(), ['bot1', 'bot2']);
  }));

  it('should filter bots based on input', fakeAsync(() => {
    const gateway = {
      ListBotConfigurations: sinon
        .stub()
        .resolves({ configurations: ['chrome-bot', 'android-bot', 'win-bot'] }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get('bot')?.setValue('bot');
    assert.deepEqual(component.filteredBots(), ['android-bot', 'chrome-bot', 'win-bot']);

    component.jobForm.get('bot')?.setValue('android');
    assert.deepEqual(component.filteredBots(), ['android-bot']);
  }));

  it('should return all bots when query is empty', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot']);
    component.jobForm.patchValue({ bot: '' });
    assert.deepEqual(component.filteredBots(), ['Chrome-Bot', 'Android-Bot', 'Win-Bot']);
  });

  it('should match multiple bots when query matches them', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot']);
    component.jobForm.patchValue({ bot: 'bot' });
    assert.deepEqual(component.filteredBots(), ['Chrome-Bot', 'Android-Bot']);
  });

  it('should trim spaces and ignore case when filtering bots', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot', 'macOS-Device']);
    component.jobForm.patchValue({ bot: '  wbt  ' });
    assert.deepEqual(component.filteredBots(), ['Win-Bot']);
  });

  it('should match to a single bot when input equal bot name', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot', 'macOS-Device']);
    component.jobForm.patchValue({ bot: 'Android-Bot' });
    assert.deepEqual(component.filteredBots(), ['Android-Bot']);
  });

  it('should fetch benchmarks on initialization', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon.stub().resolves({ benchmarks: ['bench1', 'bench2'] }),
    };
    const component = createComponent(gateway);

    component.ngOnInit();
    tick();

    assert.deepEqual(component.benchmarks(), ['bench1', 'bench2']);
    assert.deepEqual(component.filteredBenchmarks(), ['bench1', 'bench2']);
  }));

  it('should filter benchmarks based on input', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon
        .stub()
        .resolves({ benchmarks: ['speedometer3', 'jetstream2', 'rendering'] }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get('benchmark')?.setValue('meter');
    assert.deepEqual(component.filteredBenchmarks(), ['speedometer3']);

    component.jobForm.get('benchmark')?.setValue('rendering');
    assert.deepEqual(component.filteredBenchmarks(), ['rendering']);
  }));

  it('should re-validate bot when bots list is loaded', fakeAsync(() => {
    const component = createComponent({
      ListBotConfigurations: async () => ({ configurations: ['linux-perf'] }),
    });
    component.jobForm.get('bot')?.setValue('linux-perf');
    assert.isTrue(component.jobForm.get('bot')?.hasError('invalidAutocomplete'));

    component.ngOnInit();
    tick();

    assert.isFalse(component.jobForm.get('bot')?.hasError('invalidAutocomplete'));
  }));

  it('should re-validate benchmark when benchmarks list is loaded', fakeAsync(() => {
    const component = createComponent({
      ListBenchmarks: async () => ({ benchmarks: ['speedometer3'] }),
    });
    component.jobForm.get('benchmark')?.setValue('speedometer3');
    assert.isTrue(component.jobForm.get('benchmark')?.hasError('invalidAutocomplete'));

    component.ngOnInit();
    tick();

    assert.isFalse(component.jobForm.get('benchmark')?.hasError('invalidAutocomplete'));
  }));
});
