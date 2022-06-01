import './index';

import sinon from 'sinon';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { priorities } from '../task-priority-sk/test_data';
import { chromiumPatchResult } from '../patch-sk/test_data';
import { InputSk } from '../input-sk/input-sk';
import { MetricsAnalysisSk } from './metrics-analysis-sk';
import { MetricsAnalysisAddTaskVars } from '../json';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('metrics-analysis-sk', () => {
  fetchMock.config.overwriteRoutes = false;
  const factory = setUpElementUnderTest<MetricsAnalysisSk>('metrics-analysis-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks?: number) => {
    fetchMock.getOnce('begin:/_/task_priorities/', priorities);
    mockActiveTasks(activeTasks);
    const ele = factory();
    await fetchMock.flush(true);
    return ele;
  };

  // Make our test object global to make helper functions convenient.
  let metricsAnalysis: MetricsAnalysisSk;

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
    sinon.restore();
  });

  const mockActiveTasks = (n: number|undefined) => {
    n = n || 0;
    // For running tasks for the user we put a nonzero total in one of the
    // responses, and 0 in the remaining 6.
    fetchMock.postOnce('begin:/_/get', {
      data: [],
      ids: [],
      pagination: { offset: 0, size: 1, total: n },
      permissions: [],
    });
    fetchMock.post('begin:/_/get', {
      data: [],
      ids: [],
      pagination: { offset: 0, size: 1, total: 0 },
      permissions: [],
    }, { repeat: 6 });
  };

  const setDescription = (d: string) => {
    ($$('#description', metricsAnalysis) as InputSk).value = d;
  };

  const setMetricName = (m: string) => {
    ($$('#metric_name', metricsAnalysis) as InputSk).value = m;
  };

  const setAnalysisTaskId = (a: string) => {
    ($$('#analysis_task_id', metricsAnalysis) as InputSk).value = a;
  };

  const setPatch = async (patchtype: string, value: string, response: any) => {
    fetchMock.postOnce('begin:/_/cl_data', response);
    const input = $$(`#${patchtype}_patch input-sk`) as InputSk;
    input.value = value;
    input.dispatchEvent(new Event('input', {
      bubbles: true,
    }));
    await fetchMock.flush(true);
  };

  const clickSubmit = () => {
    ($$('#submit', metricsAnalysis) as InputSk).click();
  };

  const expectTaskTriggered = () => {
    fetchMock.postOnce('begin:/_/add_metrics_analysis_task', {});
  };

  it('loads, has defaults set', async () => {
    metricsAnalysis = await newInstance();
    expect($$('#repeat_after_days', metricsAnalysis)).to.have.property('frequency', '0');
    expect($$('#task_priority', metricsAnalysis)).to.have.property('priority', '100');
    expect($$('#benchmark_args', metricsAnalysis)).to.have.property('value', '--output-format=csv');
    expect($$('#value_column_name', metricsAnalysis)).to.have.property('value', 'avg');
  });

  it('requires metric name', async () => {
    metricsAnalysis = await newInstance();
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect((err as CustomEvent).detail.message).to.equal('Please specify a metric name');
  });

  it('requires traces', async () => {
    metricsAnalysis = await newInstance();
    setMetricName('a metric');
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect((err as CustomEvent).detail.message).to.equal('Please specify an analysis task id or custom traces');
  });

  it('requires description', async () => {
    metricsAnalysis = await newInstance();
    setMetricName('a metric');
    setAnalysisTaskId('seven');
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect((err as CustomEvent).detail.message).to.equal('Please specify a description');
  });

  it('rejects bad patch', async () => {
    metricsAnalysis = await newInstance();
    setDescription('Testing things');
    setMetricName('a metric');
    await setPatch('chromium', '1234', { cl: '1234' }); // Patch result is bogus.
    let event = eventPromise('error-sk');
    clickSubmit();
    let err = await event;
    expect((err as CustomEvent).detail.message).to.contain('Unable to fetch chromium patch from CL 1234');

    await setPatch('chromium', '1234', {}); // CL doesn't load.
    event = eventPromise('error-sk');
    clickSubmit();
    err = await event;
    expect((err as CustomEvent).detail.message).to.contain('Unable to load chromium CL 1234');
  });

  it('triggers a new task', async () => {
    metricsAnalysis = await newInstance();
    setMetricName('foo');
    setAnalysisTaskId('bar');
    await setPatch('chromium', '1234', chromiumPatchResult);
    metricsAnalysis._gotoRunsHistory = () => {
      // Karma can't handlje page reloads, so disable it.
    };
    expectTaskTriggered();
    sinon.stub(window, 'confirm').returns(true);
    clickSubmit();
    await fetchMock.flush(true);
    const taskJson = JSON.parse(fetchMock.lastOptions()!.body as any) as MetricsAnalysisAddTaskVars;
    // Here we test the 'interesting' arguments. We try a single patch,
    // and we don't bother filling in the simple string arguments.
    const expectation = {
      analysis_task_id: 'bar',
      custom_traces: '',
      benchmark_args: '--output-format=csv',
      desc: 'Testing https://chromium-review.googlesource.com/c/2222715/3 (Roll Skia from cc7ec24ca824 to 1dbc3b533962 (3 revisions))',
      chromium_patch: '\n\ndiff --git a/DEPS b/DEPS\nindex 849ae22..ee07579 100644\n--- a/DEPS\n+++ b/DEPS\n@@ -178,7 +178,7 @@\n   # Three lines of non-changing comments so that\n   # the commit queue can handle CLs rolling Skia\n   # and whatever else without interference from each other.\n-  \'skia_revision\': \'cc7ec24ca824ca13d5a8a8e562fcec695ae54390\',\n+  \'skia_revision\': \'1dbc3b533962b0ae803a2a5ee89f61146228d73b\',\n   # Three lines of non-changing comments so that\n   # the commit queue can handle CLs rolling V8\n   # and whatever else without interference from each other.\n',
      catapult_patch: '',
      metric_name: 'foo',
      repeat_after_days: '0',
      task_priority: '100',
      value_column_name: 'avg',
    };
    expect(taskJson).to.deep.equal(expectation);
  });

  it('loads with a template', async () => {
    metricsAnalysis = await newInstance();
    const mockAddTaskVars: MetricsAnalysisAddTaskVars = {
      metric_name: 'test metric name',
      analysis_task_id: 'test task id',
      custom_traces: 'google.com,youtube.com',
      benchmark_args: 'test benchmark args',
      value_column_name: 'avg2',
      desc: 'test description',
      chromium_patch: 'test chromium patch',
      catapult_patch: 'test catapult patch',
      repeat_after_days: '7',
      task_priority: '110',
      cc_list: ['superman@krypton.com', 'batman@gotham.com'],
      analysis_output_link: 'test',
      ts_added: '20190314202843',
      username: 'superman@krypton.com',
    };
    fetchMock.post('begin:/_/edit_metrics_analysis_task', mockAddTaskVars);
    metricsAnalysis.handleTemplateID('123');
    await fetchMock.flush(true);
    expect($$('#description', metricsAnalysis)).to.have.property('value', 'test description');
    expect($$('#analysis_task_id', metricsAnalysis)).to.have.property('value', 'test task id');
    expect($$('#repeat_after_days', metricsAnalysis)).to.have.property('frequency', '7');
    expect($$('#task_priority', metricsAnalysis)).to.have.property('priority', '110');
    expect($$('#value_column_name', metricsAnalysis)).to.have.property('value', 'avg2');
  });

  it('rejects if too many active tasks', async () => {
    // Report user as having 4 active tasks.
    metricsAnalysis = await newInstance(4);
    setDescription('Testing things');
    setMetricName('a benchmark');
    setAnalysisTaskId('seven');
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect((err as CustomEvent).detail.message).to.contain('You have 4 currently running tasks');
  });
});
