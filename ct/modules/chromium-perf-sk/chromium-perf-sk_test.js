import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { benchmarks, platforms } from './test_data';
import { pageSets } from '../pageset-selector-sk/test_data';
import { priorities } from '../task-priority-sk/test_data';
import { chromiumPatchResult } from '../patch-sk/test_data';
import { 
  eventPromise,
  setUpElementUnderTest
} from '../../../infra-sk/modules/test_util';

describe('chromium-perf-sk', () => {
  const factory = setUpElementUnderTest('chromium-perf-sk');
  // Returns a new element with the pagesets, task priorirites, and
  // active tasks fetches complete, and benchmarks and platforms set.
  const newInstance = async (activeTasks, init) => {
    fetchMock.postOnce('begin:/_/page_sets/', pageSets);
    fetchMock.getOnce('begin:/_/task_priorities/', priorities);
   // fetchMock.postOnce('begin:/_/cl_data', chromiumPatchResult);
    mockActiveTasks(activeTasks);
    const wrappedInit = (ele) => {
      if (init) {
        init(ele);
      }
      ele.benchmarks = benchmarks;
      ele.platforms = platforms;
    };
    const ele = factory(wrappedInit);
    //await new Promise((resolve) => setTimeout(resolve, 0));
    await fetchMock.flush(true);
    return ele;
  };
  fetchMock.config.overwriteRoutes = false;

  let chromiumPerf;
  beforeEach(() => {
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  const mockActiveTasks = (n) => {
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

  const setDescription = (d) => {
    $$('#description', chromiumPerf).value = d;
  };

  const setBenchmark = (b) => {
    $$('#benchmark_name', chromiumPerf).value = b;
  };

  const setPatch = async (patchtype, value, response) => {
    fetchMock.postOnce('begin:/_/cl_data', response);
    const input = $$(`#${patchtype}_patch input-sk`);
    input.value = value;
    input.dispatchEvent(new Event('input', {
      bubbles: true,
    }));
    await fetchMock.flush(true);
  };

  const clickSubmit = () => {
    $$('#submit', chromiumPerf).click();
  };

  const expectTaskTriggered = () => {
    fetchMock.postOnce('begin:/_/add_chromium_perf_task', {});
  };

  it('loads, has defaults set', async () => {
    chromiumPerf = await newInstance();
    expect(chromiumPerf.platforms[$$('#platform_selector', chromiumPerf)
      .selection]).to.equal('Linux (Ubuntu18.04 machines)');
    expect($$('#pageset_selector', chromiumPerf)).to.have.property('selected', '10k');
    expect($$('#pageset_selector', chromiumPerf)).to.have.property('customPages', '');
    expect($$('#repeat_after_days', this)).to.have.property('frequency', '0');
    expect($$('#task_priority', this)).to.have.property('priority', '100');
    expect($$('#benchmark_args', this)).to.have.property('value',
      '--output-format=csv --pageset-repeat=1 '
      + '--skip-typ-expectations-tags-validation --legacy-json-trace-format');
    expect($$('#value_column_name', this)).to.have.property('value', 'avg');
  });

  it('requires description', async () => {
    chromiumPerf = await newInstance();
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect(err.detail.message).to.equal('Please specify a description');
  });

  it('requires benchmark', async () => {
    chromiumPerf = await newInstance();
    setDescription('Testing things');
    const event = eventPromise('error-sk');
    clickSubmit();
    const err = await event;
    expect(err.detail.message).to.equal('Please specify a benchmark');
  });

  it('rejects bad patch', async () => {
    chromiumPerf = await newInstance();
    setDescription('Testing things');
    setBenchmark('a benchmark');
    await setPatch('skia', '1234', { cl: '1234' }); // Patch result is bogus.
    let event = eventPromise('error-sk');
    clickSubmit();
    let err = await event;
    expect(err.detail.message).to.contain('Unable to fetch skia patch from CL 1234');

    await setPatch('skia', '1234', {}); // CL doesn't load.
    event = eventPromise('error-sk');
    clickSubmit();
    err = await event;
    expect(err.detail.message).to.contain('Unable to load skia CL 1234');
  });

  it('triggers a new task', async () => {
    chromiumPerf = await newInstance();
    setDescription('Testing things');
    setBenchmark('a benchmark');
    await setPatch('chromium', '1234', chromiumPatchResult);
    expectTaskTriggered();
    clickSubmit();
    $('#confirm_dialog button')[1].click(); // brittle way to press 'ok'
    await fetchMock.flush(true);
    const taskJson = fetchMock.lastOptions();
    console.log(taskJson);
  });


/*
  it('delete option shown', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', false);
  });

  it('delete option hidden', async () => {
    const table = await loadTableWithReplies([singleResultNoDelete]);

    expect($$('delete-icon-sk', table)).to.have.property('hidden', true);
  });

  it('delete flow works', async () => {
    const table = await loadTableWithReplies([singleResultCanDelete]);

    expect($$('dialog', table)).to.have.property('open', false);
    $$('delete-icon-sk', table).click();
    expect($$('dialog', table)).to.have.property('open', true);
    fetchMock.postOnce((url, options) => url.startsWith('/_/delete_') && options.body === JSON.stringify({ id: 1 }), 200);
    // TODO(weston): Update common-sk/confirm-dialog-sk to make this less
    // brittle.
    $$('dialog', table).querySelectorAll('button')[1].click();
    expect($$('dialog', table)).to.have.property('open', false);
  });

  it('task details works', async () => {
    const table = await loadTableWithReplies([resultSetOneItem]);

    expect($$('.dialog-background', table)).to.have.class('hidden');
    $$('.details', table).click();

    expect($$('.dialog-background', table)).to.not.have.class('hidden');
  });*/
});
