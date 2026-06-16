import 'zone.js';
import 'zone.js/testing';
import '@angular/compiler';
import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { HttpErrorResponse } from '@angular/common/http';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import { NewJobComponent, Field } from './new-job.component';
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
      GetBenchmark: async (req) => ({
        stories: [],
        storyTags: [],
        benchmark: req?.benchmark || '',
      }),
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
    component.jobForm.get(Field.Bot)?.setValue('linux-perf');
    component.benchmarks.set(['speedometer']);
    component.jobForm.get(Field.Benchmark)?.setValue('speedometer');
    component.jobForm.get(Field.Story)?.setValue('Speedometer3');
    component.jobForm.get([Field.Baseline, Field.Commit])?.setValue('abcd1234');
    return component;
  }

  it('should initialize form with default values', () => {
    const component = createComponent();
    assert.isNotNull(component.jobForm);
    assert.equal(component.jobForm.get(Field.Attempts)?.value, 30);
    assert.equal(component.jobForm.get([Field.Baseline, Field.Commit])?.value, '');
    assert.equal(component.jobForm.get([Field.Experiment, Field.Commit])?.value, '');
    assert.isFalse(component.jobForm.valid);
  });

  it('should create a valid form', () => {
    const form = createValidComponent().jobForm;
    assert.isTrue(form.valid);
  });

  it('should validate bot', () => {
    const form = createValidComponent().jobForm;
    form.get(Field.Bot)?.setValue('');
    assert.isFalse(form.valid);
  });

  it('should validate bot autocomplete values', () => {
    const component = createComponent();
    component.bots.set(['linux-perf', 'win-perf']);

    component.jobForm.get(Field.Bot)?.setValue('linux-perf');
    assert.isTrue(component.jobForm.get(Field.Bot)?.valid);

    component.jobForm.get(Field.Bot)?.setValue('unknown-bot');
    assert.isTrue(component.jobForm.get(Field.Bot)?.hasError('invalidAutocomplete'));
  });

  it('should validate benchmark autocomplete values', () => {
    const component = createComponent();
    component.benchmarks.set(['speedometer3', 'jetstream2']);

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    assert.isTrue(component.jobForm.get(Field.Benchmark)?.valid);

    component.jobForm.get(Field.Benchmark)?.setValue('unknown-benchmark');
    assert.isTrue(component.jobForm.get(Field.Benchmark)?.hasError('invalidAutocomplete'));
  });

  it('should validate attempts count', () => {
    const form = createValidComponent().jobForm;
    form.get(Field.Attempts)?.setValue(0);
    assert.isFalse(form.valid);

    form.get(Field.Attempts)?.setValue(-5);
    assert.isFalse(form.valid);

    form.get(Field.Attempts)?.setValue(1);
    assert.isTrue(form.valid);
  });

  it('should validate bug ID', () => {
    const form = createValidComponent().jobForm;
    form.get(Field.BugId)?.setValue('');
    assert.isTrue(form.valid);

    form.get(Field.BugId)?.setValue(0);
    assert.isFalse(form.valid);

    form.get(Field.BugId)?.setValue(-123);
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

    component.jobForm.get(Field.Bot)?.setValue('bot');
    assert.deepEqual(component.filteredBots(), ['android-bot', 'chrome-bot', 'win-bot']);

    component.jobForm.get(Field.Bot)?.setValue('android');
    assert.deepEqual(component.filteredBots(), ['android-bot']);
  }));

  it('should return all bots when query is empty', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot']);
    component.jobForm.patchValue({ [Field.Bot]: '' });
    assert.deepEqual(component.filteredBots(), ['Chrome-Bot', 'Android-Bot', 'Win-Bot']);
  });

  it('should match multiple bots when query matches them', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot']);
    component.jobForm.patchValue({ [Field.Bot]: 'bot' });
    assert.deepEqual(component.filteredBots(), ['Chrome-Bot', 'Android-Bot']);
  });

  it('should trim spaces and ignore case when filtering bots', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot', 'macOS-Device']);
    component.jobForm.patchValue({ [Field.Bot]: '  wbt  ' });
    assert.deepEqual(component.filteredBots(), ['Win-Bot']);
  });

  it('should match to a single bot when input equal bot name', () => {
    const component = createComponent();
    component.bots.set(['Chrome-Bot', 'Android-Bot', 'Win-Bot', 'macOS-Device']);
    component.jobForm.patchValue({ [Field.Bot]: 'Android-Bot' });
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

    component.jobForm.get(Field.Benchmark)?.setValue('meter');
    assert.deepEqual(component.filteredBenchmarks(), ['speedometer3']);

    component.jobForm.get(Field.Benchmark)?.setValue('rendering');
    assert.deepEqual(component.filteredBenchmarks(), ['rendering']);
  }));

  it('should re-validate bot when bots list is loaded', fakeAsync(() => {
    const component = createComponent({
      ListBotConfigurations: async () => ({ configurations: ['linux-perf'] }),
    });
    component.jobForm.get(Field.Bot)?.setValue('linux-perf');
    assert.isTrue(component.jobForm.get(Field.Bot)?.hasError('invalidAutocomplete'));

    component.ngOnInit();
    tick();

    assert.isFalse(component.jobForm.get(Field.Bot)?.hasError('invalidAutocomplete'));
  }));

  it('should re-validate benchmark when benchmarks list is loaded', fakeAsync(() => {
    const component = createComponent({
      ListBenchmarks: async () => ({ benchmarks: ['speedometer3'] }),
    });
    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    assert.isTrue(component.jobForm.get(Field.Benchmark)?.hasError('invalidAutocomplete'));

    component.ngOnInit();
    tick();

    assert.isFalse(component.jobForm.get(Field.Benchmark)?.hasError('invalidAutocomplete'));
  }));

  it('should fetch benchmark details when benchmark selection changes to a valid value', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon.stub().resolves({ benchmarks: ['speedometer3'] }),
      GetBenchmark: sinon.stub().resolves({
        stories: ['story1', 'story2'],
        storyTags: ['tag1'],
        benchmark: 'speedometer3',
      }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    tick();

    assert.isTrue(gateway.GetBenchmark.calledWith({ benchmark: 'speedometer3' }));
    assert.deepEqual(component.stories(), ['story1', 'story2']);
    assert.deepEqual(component.storyTags(), ['tag1']);
  }));

  it('should reset stories and story tags when benchmark changes to invalid/empty', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon.stub().resolves({ benchmarks: ['speedometer3'] }),
      GetBenchmark: sinon.stub().resolves({
        stories: ['story1', 'story2'],
        storyTags: ['tag1'],
        benchmark: 'speedometer3',
      }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    tick();
    assert.deepEqual(component.stories(), ['story1', 'story2']);

    component.jobForm.get(Field.Benchmark)?.setValue('');
    tick();
    assert.deepEqual(component.stories(), []);
    assert.deepEqual(component.storyTags(), []);
    assert.equal(component.jobForm.get(Field.Story)?.value, '');
    assert.equal(component.jobForm.get(Field.StoryTags)?.value, '');
  }));

  it('should clear story and tags if they are not in the new benchmark details', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon.stub().resolves({ benchmarks: ['speedometer3', 'speedometer4'] }),
      GetBenchmark: sinon
        .stub()
        .withArgs({ benchmark: 'speedometer3' })
        .resolves({
          stories: ['story1'],
          storyTags: ['tag1'],
          benchmark: 'speedometer3',
        })
        .withArgs({ benchmark: 'speedometer4' })
        .resolves({
          stories: ['story2'],
          storyTags: ['tag2'],
          benchmark: 'speedometer4',
        }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    tick();
    component.jobForm.get(Field.Story)?.setValue('story1');
    component.jobForm.get(Field.StoryTags)?.setValue('tag1');
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer4');
    tick();

    assert.equal(component.jobForm.get(Field.Story)?.value, '');
    assert.equal(component.jobForm.get(Field.StoryTags)?.value, '');
  }));

  it('should persist story and tags if they are in the new benchmark details', fakeAsync(() => {
    const gateway = {
      ListBenchmarks: sinon.stub().resolves({ benchmarks: ['speedometer3', 'speedometer4'] }),
      GetBenchmark: sinon
        .stub()
        .withArgs({ benchmark: 'speedometer3' })
        .resolves({
          stories: ['story1'],
          storyTags: ['tag1'],
          benchmark: 'speedometer3',
        })
        .withArgs({ benchmark: 'speedometer4' })
        .resolves({
          stories: ['story1'],
          storyTags: ['tag1'],
          benchmark: 'speedometer4',
        }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer3');
    tick();
    component.jobForm.get(Field.Story)?.setValue('story1');
    component.jobForm.get(Field.StoryTags)?.setValue('tag1');
    tick();

    component.jobForm.get(Field.Benchmark)?.setValue('speedometer4');
    tick();

    assert.equal(component.jobForm.get(Field.Story)?.value, 'story1');
    assert.equal(component.jobForm.get(Field.StoryTags)?.value, 'tag1');
  }));

  it('should filter stories and story tags based on input', fakeAsync(() => {
    const component = createComponent();
    component.stories.set(['story1', 'another-story', 'third-story']);
    component.storyTags.set(['tag1', 'another-tag']);

    component.jobForm.get(Field.Story)?.setValue('story');
    assert.deepEqual(component.filteredStories(), ['story1', 'another-story', 'third-story']);

    component.jobForm.get(Field.Story)?.setValue('another');
    assert.deepEqual(component.filteredStories(), ['another-story']);

    component.jobForm.get(Field.StoryTags)?.setValue('tag');
    assert.deepEqual(component.filteredStoryTags(), ['tag1', 'another-tag']);
  }));
});
