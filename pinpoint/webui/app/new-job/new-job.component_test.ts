import 'zone.js';
import 'zone.js/testing';
import '@angular/compiler';
import { TestBed, fakeAsync, tick } from '@angular/core/testing';
import { HttpErrorResponse } from '@angular/common/http';
import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';
import {
  NewJobComponent,
  Field,
  Variant,
  CommitOption,
  INPUT_DEBOUNCE_TIME_MS,
} from './new-job.component';
import { GatewayService } from '../gateway/gateway.service';
import { BuildInfo } from '../gateway/gateway';
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
      ListRecentBuilds: async () => ({ builds: [] }),
      GetCommit: async () => ({
        repository: '',
        gitHash: '',
        url: '',
        author: '',
        created: '',
        subject: '',
        message: '',
        commitBranch: '',
        commitPosition: 0,
        reviewUrl: '',
        changeId: '',
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
    component.jobForm.get([Variant.Baseline, Field.Commit])?.setValue('abcd1234');
    component.baselineCommitInfo.set({
      repository: 'chromium',
      gitHash: 'abcd1234',
      url: 'https://url',
      author: 'author',
      created: '',
      subject: 'subject',
      message: 'message',
      commitBranch: 'main',
      commitPosition: 100,
      reviewUrl: 'https://review',
      changeId: 'I123',
    });
    return component;
  }

  it('should initialize form with default values', () => {
    const component = createComponent();
    assert.isNotNull(component.jobForm);
    assert.equal(component.jobForm.get(Field.Attempts)?.value, 30);
    assert.equal(
      component.jobForm.get([Variant.Baseline, Field.Commit])?.value,
      CommitOption.Recent
    );
    assert.equal(component.jobForm.get([Variant.Experiment, Field.Commit])?.value, '');
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

  it('should filter baseline and experiment commits based on input query', fakeAsync(() => {
    const component = createComponent();
    const builds: BuildInfo[] = [
      { gitHash: 'abc123commit', buildNumber: 10, created: '' },
      { gitHash: 'def456commit', buildNumber: 20, created: '' },
      { gitHash: 'abc789commit', buildNumber: 30, created: '' },
    ];
    component.recentBuilds.set(builds);

    // Test baseline commit filtering by git hash
    component.jobForm.get([Variant.Baseline, Field.Commit])?.setValue('abc');
    assert.deepEqual(component.filteredBaselineCommits(), [
      { gitHash: 'abc123commit', buildNumber: 10, created: '' },
      { gitHash: 'abc789commit', buildNumber: 30, created: '' },
    ]);

    // Test baseline commit filtering by build number
    component.jobForm.get([Variant.Baseline, Field.Commit])?.setValue('2');
    assert.deepEqual(component.filteredBaselineCommits(), [
      { gitHash: 'def456commit', buildNumber: 20, created: '' },
    ]);

    // Test case insensitivity and trimming
    component.jobForm.get([Variant.Baseline, Field.Commit])?.setValue('  ABC  ');
    assert.deepEqual(component.filteredBaselineCommits(), [
      { gitHash: 'abc123commit', buildNumber: 10, created: '' },
      { gitHash: 'abc789commit', buildNumber: 30, created: '' },
    ]);

    // Test experiment commit filtering
    component.jobForm.get([Variant.Experiment, Field.Commit])?.setValue('def');
    assert.deepEqual(component.filteredExperimentCommits(), [
      { gitHash: 'def456commit', buildNumber: 20, created: '' },
    ]);
  }));

  it('should determine whether to show "Recent" and "HEAD" options based on query', fakeAsync(() => {
    const component = createComponent();
    component.ngOnInit();
    tick();

    // On load, baseline commit is "Recent", so only showBaselineRecent should be true.
    assert.isTrue(component.showBaselineRecent());
    assert.isFalse(component.showBaselineHead());

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;

    // Empty query should show both
    baselineCommit.setValue('');
    tick();
    assert.isTrue(component.showBaselineRecent());
    assert.isTrue(component.showBaselineHead());

    // Query "rec" should show "Recent" but not "HEAD"
    baselineCommit.setValue('rec');
    tick();
    assert.isTrue(component.showBaselineRecent());
    assert.isFalse(component.showBaselineHead());

    // Query "HEAD" should show "HEAD" but not "Recent"
    baselineCommit.setValue(CommitOption.Head);
    tick();
    assert.isFalse(component.showBaselineRecent());
    assert.isTrue(component.showBaselineHead());

    // Query "other" should hide both
    baselineCommit.setValue('other');
    tick();
    assert.isFalse(component.showBaselineRecent());
    assert.isFalse(component.showBaselineHead());
  }));

  it('should fetch recent commits when bot selection changes to a valid value', fakeAsync(() => {
    const rawCommits: BuildInfo[] = [
      { gitHash: 'commit1', buildNumber: 1, created: '2026-06-11T14:28:34Z' },
      { gitHash: 'commit2', buildNumber: 2, created: '2026-06-11T14:23:10Z' },
    ];
    const expectedCommits: BuildInfo[] = [
      { gitHash: 'commit2', buildNumber: 2, created: '2026-06-11T14:23:10Z' },
      { gitHash: 'commit1', buildNumber: 1, created: '2026-06-11T14:28:34Z' },
    ];
    const gateway = {
      ListBotConfigurations: sinon.stub().resolves({ configurations: ['linux-perf'] }),
      ListRecentBuilds: sinon.stub().resolves({
        builds: rawCommits,
      }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Bot)?.setValue('linux-perf');
    tick();

    assert.isTrue(gateway.ListRecentBuilds.calledWith({ configuration: 'linux-perf' }));
    assert.deepEqual(component.recentBuilds(), expectedCommits);
  }));

  it('should reset recent commits and form fields when bot changes', fakeAsync(() => {
    const rawCommits: BuildInfo[] = [
      { gitHash: 'commit1', buildNumber: 1, created: '2026-06-11T14:28:34Z' },
      { gitHash: 'commit2', buildNumber: 2, created: '2026-06-11T14:23:10Z' },
    ];
    const expectedCommits: BuildInfo[] = [
      { gitHash: 'commit2', buildNumber: 2, created: '2026-06-11T14:23:10Z' },
      { gitHash: 'commit1', buildNumber: 1, created: '2026-06-11T14:28:34Z' },
    ];
    const gateway = {
      ListBotConfigurations: sinon.stub().resolves({ configurations: ['linux-perf'] }),
      ListRecentBuilds: sinon.stub().resolves({
        builds: rawCommits,
      }),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    component.jobForm.get(Field.Bot)?.setValue('linux-perf');
    tick();
    assert.deepEqual(component.recentBuilds(), expectedCommits);

    component.jobForm.get(Field.Bot)?.setValue('');
    tick();
    assert.deepEqual(component.recentBuilds(), []);
    assert.equal(
      component.jobForm.get([Variant.Baseline, Field.Commit])?.value,
      CommitOption.Recent
    );
    assert.equal(component.jobForm.get([Variant.Experiment, Field.Commit])?.value, '');
  }));

  it('should fetch commit info and clear error on successful GetCommit', fakeAsync(() => {
    const mockCommitResponse = {
      repository: 'chromium',
      gitHash: 'abcdef0123456789abcdef0123456789abcdef01',
      url: 'http://url',
      author: 'author',
      created: '',
      subject: 'test subject',
      message: 'test message',
      commitBranch: 'main',
      commitPosition: 1234,
      reviewUrl: 'http://review',
      changeId: 'I1234',
    };
    const gateway = {
      GetCommit: sinon.stub().resolves(mockCommitResponse),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue('abcdef0123456789abcdef0123456789abcdef01');
    tick(INPUT_DEBOUNCE_TIME_MS);

    assert.isTrue(
      gateway.GetCommit.calledWith({ commit: 'abcdef0123456789abcdef0123456789abcdef01' })
    );
    assert.deepEqual(component.baselineCommitInfo(), mockCommitResponse);
    assert.isFalse(baselineCommit.hasError('invalidCommit'));
  }));

  it('should set invalidCommit error and clear commit info on failed GetCommit', fakeAsync(() => {
    const gateway = {
      GetCommit: sinon.stub().rejects(new Error('Commit not found')),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue('abcdef0123456789abcdef0123456789abcdef01');
    tick(INPUT_DEBOUNCE_TIME_MS);

    assert.isTrue(
      gateway.GetCommit.calledWith({ commit: 'abcdef0123456789abcdef0123456789abcdef01' })
    );
    assert.isNull(component.baselineCommitInfo());
    assert.isTrue(baselineCommit.hasError('invalidCommit'));
  }));

  it('should clear commit info and errors when input is empty', fakeAsync(() => {
    const component = createComponent();
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setErrors({ invalidCommit: true });
    component.baselineCommitInfo.set({} as any);

    baselineCommit.setValue('');
    tick();

    assert.isNull(component.baselineCommitInfo());
    assert.isFalse(baselineCommit.hasError('invalidCommit'));
  }));

  it('should update baseline commit info when recent builds load and "Recent" is selected', fakeAsync(() => {
    const mockCommitResponse = {
      repository: 'chromium',
      gitHash: 'hash123',
      url: 'http://url',
      author: 'author',
      created: '',
      subject: 'recent commit subject',
      message: 'recent commit message',
      commitBranch: 'main',
      commitPosition: 1234,
      reviewUrl: 'http://review',
      changeId: 'I1234',
    };
    const gateway = {
      GetCommit: sinon.stub().resolves(mockCommitResponse),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue(CommitOption.Recent);
    tick(INPUT_DEBOUNCE_TIME_MS);

    // GetCommit shouldn't have been called yet because recentBuilds is empty.
    assert.isTrue(gateway.GetCommit.notCalled);
    assert.isNull(component.baselineCommitInfo());

    // Load recent builds.
    component.recentBuilds.set([{ gitHash: 'hash123', buildNumber: 5, created: '' }]);
    tick(); // Run the effect

    assert.isTrue(gateway.GetCommit.calledWith({ commit: 'hash123' }));
    assert.deepEqual(component.baselineCommitInfo(), mockCommitResponse);
  }));

  it('should update experiment commit info when recent builds load and "Recent" is selected', fakeAsync(() => {
    const mockCommitResponse = {
      repository: 'chromium',
      gitHash: 'hash123',
      url: 'http://url',
      author: 'author',
      created: '',
      subject: 'recent commit subject',
      message: 'recent commit message',
      commitBranch: 'main',
      commitPosition: 1234,
      reviewUrl: 'http://review',
      changeId: 'I1234',
    };
    const gateway = {
      GetCommit: sinon.stub().resolves(mockCommitResponse),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const experimentCommit = component.jobForm.get([Variant.Experiment, Field.Commit])!;
    experimentCommit.setValue(CommitOption.Recent);
    tick(INPUT_DEBOUNCE_TIME_MS);

    // GetCommit shouldn't have been called yet because recentBuilds is empty.
    assert.isTrue(gateway.GetCommit.notCalled);
    assert.isNull(component.experimentCommitInfo());

    // Load recent builds.
    component.recentBuilds.set([{ gitHash: 'hash123', buildNumber: 5, created: '' }]);
    tick(); // Run the effect

    assert.isTrue(gateway.GetCommit.calledWith({ commit: 'hash123' }));
    assert.deepEqual(component.experimentCommitInfo(), mockCommitResponse);
  }));

  it('should ignore outdated commit during async call', fakeAsync(() => {
    let resolveFirst: (value: any) => void;
    const firstPromise = new Promise((resolve) => {
      resolveFirst = resolve;
    });

    const mockFirstResponse = {
      gitHash: 'abcdef0123456789abcdef0123456789abcdef01',
      subject: 'first subject',
    };
    const mockSecondResponse = {
      gitHash: 'bbcdef0123456789abcdef0123456789abcdef01',
      subject: 'second subject',
    };

    const gateway = {
      GetCommit: sinon.stub(),
    };
    gateway.GetCommit.onFirstCall().returns(firstPromise);
    gateway.GetCommit.onSecondCall().resolves(mockSecondResponse);

    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;

    baselineCommit.setValue('abcdef0123456789abcdef0123456789abcdef01');
    tick(INPUT_DEBOUNCE_TIME_MS);

    baselineCommit.setValue('bbcdef0123456789abcdef0123456789abcdef01');
    tick(INPUT_DEBOUNCE_TIME_MS);

    assert.deepEqual(component.baselineCommitInfo(), mockSecondResponse as any);

    resolveFirst!(mockFirstResponse);
    tick();

    assert.deepEqual(component.baselineCommitInfo(), mockSecondResponse as any);
  }));

  it('should debounce GetCommit calls', fakeAsync(() => {
    const gateway = {
      GetCommit: sinon.stub().resolves({}),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;

    baselineCommit.setValue('a');
    tick(100);
    baselineCommit.setValue('ab');
    tick(100);
    baselineCommit.setValue('abcdef0123456789abcdef0123456789abcdef01');
    tick(100);

    assert.isFalse(gateway.GetCommit.called);

    tick(INPUT_DEBOUNCE_TIME_MS - 100);

    assert.isTrue(gateway.GetCommit.calledOnce);
  }));

  it('should block submission if baseline commit is entered but info is not yet fetched', fakeAsync(() => {
    const gateway = {
      CreateTryJob: sinon.stub().resolves({ jobId: '12345' }),
      GetCommit: sinon.stub().resolves({ gitHash: '123' }),
      ListBotConfigurations: sinon.stub().resolves({ configurations: ['linux-perf'] }),
      ListBenchmarks: sinon.stub().resolves(['speedometer']),
    };
    const component = createValidComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue('123');

    component.submitJob();
    tick();

    assert.isFalse(gateway.CreateTryJob.called);
    assert.isTrue(baselineCommit.hasError('invalidCommit'));

    tick(INPUT_DEBOUNCE_TIME_MS);
    assert.isFalse(baselineCommit.hasError('invalidCommit'));

    component.submitJob();
    tick();
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  }));

  it('should block submission if experiment commit is entered but info is not yet fetched', fakeAsync(() => {
    const gateway = {
      CreateTryJob: sinon.stub().resolves({ jobId: '12345' }),
      GetCommit: sinon.stub().resolves({ gitHash: '456' }),
      ListBotConfigurations: sinon.stub().resolves({ configurations: ['linux-perf'] }),
      ListBenchmarks: sinon.stub().resolves(['speedometer']),
    };
    const component = createValidComponent(gateway);
    component.ngOnInit();
    tick();

    const experimentCommit = component.jobForm.get([Variant.Experiment, Field.Commit])!;
    experimentCommit.setValue('456');

    component.submitJob();
    tick();

    assert.isFalse(gateway.CreateTryJob.called);
    assert.isTrue(experimentCommit.hasError('invalidCommit'));

    tick(INPUT_DEBOUNCE_TIME_MS);
    assert.isFalse(experimentCommit.hasError('invalidCommit'));

    component.submitJob();
    tick();
    assert.isTrue(gateway.CreateTryJob.calledOnce);
  }));

  it('should fetch commit info for recent build when commit field is set to Recent', fakeAsync(() => {
    const mockCommitResponse = {
      repository: 'chromium',
      gitHash: 'hash123',
      url: 'http://url',
      author: 'author',
      created: '',
      subject: 'recent commit subject',
      message: 'recent commit message',
      commitBranch: 'main',
      commitPosition: 1234,
      reviewUrl: 'http://review',
      changeId: 'I1234',
    };
    const gateway = {
      GetCommit: sinon.stub().resolves(mockCommitResponse),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    // Populate recent builds first.
    component.recentBuilds.set([{ gitHash: 'hash123', buildNumber: 5, created: '' }]);
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue(CommitOption.Recent);
    tick(INPUT_DEBOUNCE_TIME_MS);

    assert.isTrue(gateway.GetCommit.calledWith({ commit: 'hash123' }));
    assert.deepEqual(component.baselineCommitInfo(), mockCommitResponse);
  }));

  it('should fetch commit info for HEAD when commit field is set to HEAD', fakeAsync(() => {
    const mockCommitResponse = {
      repository: 'chromium',
      gitHash: 'head_hash',
      url: 'http://url',
      author: 'author',
      created: '',
      subject: 'head commit subject',
      message: 'head commit message',
      commitBranch: 'main',
      commitPosition: 1235,
      reviewUrl: 'http://review',
      changeId: 'I5678',
    };
    const gateway = {
      GetCommit: sinon.stub().resolves(mockCommitResponse),
    };
    const component = createComponent(gateway);
    component.ngOnInit();
    tick();

    const baselineCommit = component.jobForm.get([Variant.Baseline, Field.Commit])!;
    baselineCommit.setValue(CommitOption.Head);
    tick(INPUT_DEBOUNCE_TIME_MS);

    assert.isTrue(gateway.GetCommit.calledWith({ commit: CommitOption.Head }));
    assert.deepEqual(component.baselineCommitInfo(), mockCommitResponse);
  }));

  describe('experimentPanelError', () => {
    it('should return false if experiment commit has no error', () => {
      const component = createComponent();
      assert.isFalse(component.experimentPanelError());
    });

    it('should return true if experiment commit has invalidCommit error', () => {
      const component = createComponent();
      const experimentCommit = component.jobForm.get([Variant.Experiment, Field.Commit])!;
      experimentCommit.setErrors({ invalidCommit: true });
      assert.isTrue(component.experimentPanelError());
    });
  });
});
