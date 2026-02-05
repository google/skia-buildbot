import './index';
import { expect } from 'chai';
import { TestPickerSk } from './test-picker-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { NextParamListHandlerResponse, NextParamListHandlerRequest } from '../json';
import { toParamSet } from '../../../infra-sk/modules/query';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';

describe('test-picker-sk', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');

  let element: TestPickerSk;
  let fetchMock: any;

  beforeEach(async () => {
    // Mock the fetch function.
    fetchMock = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};
      if (Object.keys(params).length === 0) {
        paramset['benchmark'] = ['benchmark1', 'benchmark2'];
      } else if (params.benchmark) {
        paramset['bot'] = ['bot1', 'bot2'];
      }
      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    window.fetch = fetchMock;

    element = newInstance((_el: TestPickerSk) => {});
    element.initializeTestPicker(['benchmark', 'bot', 'test'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  it('should create the first field on initialization', () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    expect(field).to.not.equal(null);
    expect(field!.label).to.equal('benchmark');
  });

  it('should create a new field when a value is selected', async () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['benchmark1'] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));
    const fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    expect(fields.length).to.equal(2);
    expect(fields[1].label).to.equal('bot');
  });

  it('should remove child fields when a value is cleared', async () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['benchmark1'] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));
    let fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    expect(fields.length).to.equal(2);

    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: [] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));
    fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    expect(fields.length).to.equal(1);
  });

  it('should emit a plot-button-clicked event', async () => {
    const plotButton = element.querySelector<HTMLButtonElement>('#plot-button');
    plotButton!.disabled = false;
    const eventPromise = new Promise<CustomEvent>((resolve) => {
      element.addEventListener('plot-button-clicked', (e) => {
        resolve(e as CustomEvent);
      });
    });
    plotButton!.click();
    const e = await eventPromise;
    expect(e.detail.query).to.equal('');
  });
});

describe('test-picker-sk conditional defaults', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');

  let element: TestPickerSk;
  let mockExplore: HTMLElement;

  beforeEach(async () => {
    // Mock explore-multi-sk in document
    mockExplore = document.createElement('explore-multi-sk');
    document.body.appendChild(mockExplore);

    // Mock fetch
    window.fetch = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};

      // Setup for: metric -> stat
      if (Object.keys(params).length === 0) {
        paramset['metric'] = ['timeNs', 'other'];
      } else if (params.metric) {
        paramset['stat'] = ['min', 'max', 'avg'];
      }

      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };

    element = newInstance((_el: TestPickerSk) => {});
    // We don't initialize here, let individual tests do it if they need specific defaults first
  });

  afterEach(() => {
    if (document.body.contains(mockExplore)) {
      document.body.removeChild(mockExplore);
    }
  });

  it('should apply conditional defaults', async () => {
    (mockExplore as any).defaults = {
      conditional_defaults: [
        {
          trigger: { param: 'metric', values: ['timeNs'] },
          apply: [{ param: 'stat', values: ['min'], select_only_first: true }],
        },
      ],
    };

    element.initializeTestPicker(['metric', 'stat'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));

    const metricField = element.querySelector<PickerFieldSk>('picker-field-sk');
    expect(metricField!.label).to.equal('metric');

    // Select 'timeNs'
    metricField!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['timeNs'] },
      })
    );

    // Wait for async operations (fetchExtraOptions + applyConditionalDefaults)
    await new Promise((resolve) => setTimeout(resolve, 200));

    const fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    expect(fields.length).to.equal(2);
    expect(fields[1].label).to.equal('stat');
    expect(fields[1].selectedItems).to.deep.equal(['min']);
  });

  it('should auto-select priority metric', async () => {
    (mockExplore as any).defaults = {
      default_trigger_priority: {
        metric: ['timeNs'],
      },
    };

    // Re-initialize to trigger auto-selection on first field
    element.initializeTestPicker(['metric', 'stat'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 200));

    const metricField = element.querySelector<PickerFieldSk>('picker-field-sk');
    expect(metricField!.selectedItems).to.deep.equal(['timeNs']);
  });
});

describe('test-picker-sk default option inference', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');

  let element: TestPickerSk;
  let fetchMock: any;

  beforeEach(async () => {
    // Mock the fetch function.
    fetchMock = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};
      let count = 0;

      // Initial load
      if (Object.keys(params).length === 0) {
        paramset['benchmark'] = ['b1'];
        count = 10;
      }
      // Select benchmark=b1
      else if (params.benchmark && !params.bot) {
        // Here we simulate the scenario:
        // We have 3 traces:
        // 1. benchmark=b1, bot=bot1
        // 2. benchmark=b1, bot=bot2
        // 3. benchmark=b1 (no bot)

        // Count should be 3.
        // paramset for 'bot' should be ['bot1', 'bot2', ''].
        paramset['bot'] = ['bot1', 'bot2', ''];
        count = 3;
      }

      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: count,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    window.fetch = fetchMock;

    element = newInstance((_el: TestPickerSk) => {});
    element.initializeTestPicker(['benchmark', 'bot'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  it('should show (default) option when backend returns empty string', async function () {
    this.timeout(5000);
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    expect(field!.label).to.equal('benchmark');

    // Select 'b1'
    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['b1'] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    const fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    expect(fields.length).to.equal(2);
    const botField = fields[1];
    expect(botField.label).to.equal('bot');

    expect(botField.options).to.include('Default');
  });

  it('should generate query with empty string when (default) is selected', async () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    // Select 'b1'
    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['b1'] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    const fields = element.querySelectorAll<PickerFieldSk>('picker-field-sk');
    const botField = fields[1];

    // Select 'Default'
    botField.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['Default'] },
      })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Verify query
    const query = element.createQueryFromFieldData();
    // Expect benchmark=b1&bot=__missing__
    expect(query).to.contain('benchmark=b1');
    expect(query).to.contain('bot=__missing__');
    // Ensure it doesn't contain 'bot=Default'
    expect(query).to.not.contain('bot=Default');
  });

  it('should remove __missing__ from query when Default is deselected', async () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    field!.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const botField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];

    // Select Default
    botField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['Default'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Deselect Default
    botField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: [] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const query = element.createQueryFromFieldData();
    expect(query).to.contain('benchmark=b1');
    expect(query).to.not.contain('bot=__missing__');
  });

  it('should include value and __missing__ when Default and other are selected', async () => {
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    field!.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const botField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];

    // Select Default AND bot1
    botField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'bot1'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    const query = element.createQueryFromFieldData();
    expect(query).to.contain('benchmark=b1');
    expect(query).to.contain('bot=__missing__');
    expect(query).to.contain('bot=bot1');
  });
});

describe('removeItemFromChart', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');
  let element: TestPickerSk;

  beforeEach(async () => {
    // Mock fetch for initialize
    window.fetch = (_url: RequestInfo | URL, _request: RequestInit | undefined) => {
      const response: NextParamListHandlerResponse = {
        paramset: { benchmark: ['b1'] } as any,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    element = newInstance((_el: TestPickerSk) => {});
    await element.initializeTestPicker(['benchmark'], {}, false);
  });

  it('removes value from fieldInfo', async () => {
    // Simulate user selection via event
    const field = element.querySelector<PickerFieldSk>('picker-field-sk');
    field!.dispatchEvent(
      new CustomEvent('value-changed', {
        detail: { value: ['b1'] },
      })
    );
    // Wait for the handler to process
    await new Promise((resolve) => setTimeout(resolve, 0));

    // Verify it is set
    expect(element.createParamSetFromFieldData()['benchmark']).to.deep.equal(['b1']);

    element.removeItemFromChart('benchmark', ['b1']);

    // Wait for async update from removeItemFromChart
    await new Promise((resolve) => setTimeout(resolve, 0));

    const paramSet = element.createParamSetFromFieldData();
    expect(paramSet['benchmark'] || []).to.be.empty;
  });
});

describe('Complex Scenarios (Default, pgo, ref, subtests)', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');
  let element: TestPickerSk;
  let fetchMock: any;

  beforeEach(async () => {
    // Mock fetch for nextParamList
    fetchMock = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};

      // Hierarchy: benchmark -> bot -> subtest_1 -> subtest_2
      if (params.subtest_1) {
        paramset['subtest_2'] = ['s2_x', 's2_y'];
      } else if (params.bot) {
        paramset['subtest_1'] = ['s1_a', 's1_b'];
      } else if (params.benchmark) {
        // bot can be: Default (implicit empty), ref, pgo, normal
        // Backend returns: ['ref', 'pgo', 'normal', '']
        paramset['bot'] = ['ref', 'pgo', 'normal', ''];
      } else {
        paramset['benchmark'] = ['b1'];
      }

      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    window.fetch = fetchMock;

    element = newInstance((_el: TestPickerSk) => {});
    // Initialize with the hierarchy
    await element.initializeTestPicker(['benchmark', 'bot', 'subtest_1', 'subtest_2'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  it('Scenario: Default -> Add ref -> Add pgo -> Remove Default', async () => {
    // 1. Select benchmark=b1
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const botField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];
    expect(botField.label).to.equal('bot');
    expect(botField.options).to.include('Default');
    expect(botField.options).to.include('ref');
    expect(botField.options).to.include('pgo');

    // 2. Select 'Default'
    botField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['Default'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));
    let query = element.createQueryFromFieldData();
    expect(query).to.contain('bot=__missing__');
    expect(query).to.not.contain('bot=ref');

    // 3. Add 'ref' (Multiselect)
    botField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'ref'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));
    query = element.createQueryFromFieldData();
    expect(query).to.contain('bot=__missing__');
    expect(query).to.contain('bot=ref');

    // 4. Add 'pgo'
    botField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'ref', 'pgo'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));
    query = element.createQueryFromFieldData();
    expect(query).to.contain('bot=__missing__');
    expect(query).to.contain('bot=ref');
    expect(query).to.contain('bot=pgo');

    // 5. Remove 'Default'
    botField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['ref', 'pgo'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));
    query = element.createQueryFromFieldData();
    expect(query).to.not.contain('bot=__missing__');
    expect(query).to.contain('bot=ref');
    expect(query).to.contain('bot=pgo');
  });

  it('Scenario: Adding subtests', async () => {
    // Select benchmark=b1
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Select bot=normal
    const botField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];
    botField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['normal'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Select subtest_1=s1_a
    const s1Field = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[2];
    expect(s1Field.label).to.equal('subtest_1');
    s1Field.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['s1_a'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Select subtest_2=s2_x
    const s2Field = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[3];
    expect(s2Field.label).to.equal('subtest_2');
    s2Field.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['s2_x'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const query = element.createQueryFromFieldData();
    expect(query).to.contain('benchmark=b1');
    expect(query).to.contain('bot=normal');
    expect(query).to.contain('subtest_1=s1_a');
    expect(query).to.contain('subtest_2=s2_x');
  });
});

describe('test-picker-sk graph interaction', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');
  let element: TestPickerSk;
  let fetchMock: any;
  let graphContainer: HTMLDivElement;

  beforeEach(async () => {
    // Setup graph container to simulate active graph
    if (!document.getElementById('graphContainer')) {
      graphContainer = document.createElement('div');
      graphContainer.id = 'graphContainer';
      graphContainer.appendChild(document.createElement('div'));
      document.body.appendChild(graphContainer);
    } else {
      graphContainer = document.getElementById('graphContainer') as HTMLDivElement;
    }

    // Mock fetch
    fetchMock = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};

      if (Object.keys(params).length === 0) {
        paramset['benchmark'] = ['b1'];
      } else if (params.benchmark) {
        paramset['subtest_4'] = ['pgo', 'ref', 'Default'];
      }

      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    window.fetch = fetchMock;

    element = newInstance((_el: TestPickerSk) => {});
    await element.initializeTestPicker(['benchmark', 'subtest_4'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  afterEach(() => {
    if (graphContainer && document.body.contains(graphContainer)) {
      document.body.removeChild(graphContainer);
    }
  });

  it('Scenario: Default -> Add pgo -> Add ref (verifies add-to-graph events)', async () => {
    // Select benchmark
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const subtestField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];
    expect(subtestField.label).to.equal('subtest_4');

    // Check autoAddTrace status.
    expect(element.autoAddTrace).to.be.true;

    let events: CustomEvent[] = [];
    element.addEventListener('add-to-graph', (e) => {
      events.push(e as CustomEvent);
    });

    // 1. Select 'Default'
    subtestField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    expect(events.length).to.equal(1);
    expect(events[0].detail.value).to.deep.equal(['__missing__']);
    events = [];

    // 2. Add 'pgo' (Value becomes ['Default', 'pgo'])
    subtestField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'pgo'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    expect(events.length).to.equal(1);
    expect(events[0].detail.value).to.deep.equal(['__missing__', 'pgo']);
    events = [];

    // 3. Add 'ref' (Value becomes ['Default', 'pgo', 'ref'])
    subtestField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'pgo', 'ref'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    expect(events.length).to.equal(1);
    expect(events[0].detail.value).to.deep.equal(['__missing__', 'pgo', 'ref']);
  });

  it('Scenario: Empty vs Default vs Multiple (Query Check)', async () => {
    // Select benchmark
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const subtestField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];

    // 1. Initial State: subtest_4 empty.
    let query = element.createQueryFromFieldData();
    expect(query).to.contain('benchmark=b1');
    expect(query).to.not.contain('subtest_4');

    // 2. Select 'Default'
    subtestField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    query = element.createQueryFromFieldData();
    expect(query).to.contain('subtest_4=__missing__');
    expect(query).to.not.contain('subtest_4=pgo');

    // 3. Add 'pgo'
    subtestField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['Default', 'pgo'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    query = element.createQueryFromFieldData();
    expect(query).to.contain('subtest_4=__missing__');
    expect(query).to.contain('subtest_4=pgo');
  });
});

describe('Comprehensive Interaction Scenarios', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');
  let element: TestPickerSk;
  let fetchMock: any;
  let graphContainer: HTMLDivElement;

  beforeEach(async () => {
    // Setup graph container
    if (!document.getElementById('graphContainer')) {
      graphContainer = document.createElement('div');
      graphContainer.id = 'graphContainer';
      graphContainer.appendChild(document.createElement('div'));
      document.body.appendChild(graphContainer);
    } else {
      graphContainer = document.getElementById('graphContainer') as HTMLDivElement;
    }

    // Mock fetch
    fetchMock = (_url: RequestInfo | URL, request: RequestInit | undefined) => {
      const req = JSON.parse(request!.body as string) as NextParamListHandlerRequest;
      const params = toParamSet(req.q!);
      const paramset: any = {};

      // Basic hierarchy
      if (Object.keys(params).length === 0) {
        paramset['benchmark'] = ['b1'];
      } else if (params.benchmark) {
        paramset['config'] = ['8888', 'gms', 'Default'];
      } else if (params.config) {
        paramset['test'] = ['t1', 't2'];
      }

      const response: NextParamListHandlerResponse = {
        paramset: paramset,
        count: 10,
      };
      return Promise.resolve(new Response(JSON.stringify(response)));
    };
    window.fetch = fetchMock;

    element = newInstance((_el: TestPickerSk) => {});
    await element.initializeTestPicker(['benchmark', 'config', 'test'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  afterEach(() => {
    if (graphContainer && document.body.contains(graphContainer)) {
      document.body.removeChild(graphContainer);
    }
  });

  it('Select All checkbox selects all options', async () => {
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const configField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];

    // Check initial state
    expect(configField.selectedItems).to.be.empty;

    // Find Select All checkbox
    const selectAllBox = configField.querySelector('checkbox-sk#select-all') as any;

    // Simulate click
    selectAllBox.checked = true;
    selectAllBox.dispatchEvent(new Event('change'));
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Verify all selected
    expect(configField.selectedItems.length).to.equal(3);
    expect(configField.selectedItems).to.include('8888');
    expect(configField.selectedItems).to.include('gms');
    expect(configField.selectedItems).to.include('Default');
  });

  it('Split checkbox triggers split event', async () => {
    const benchField = element.querySelector<PickerFieldSk>('picker-field-sk')!;
    benchField.dispatchEvent(new CustomEvent('value-changed', { detail: { value: ['b1'] } }));
    await new Promise((resolve) => setTimeout(resolve, 100));

    const configField = element.querySelectorAll<PickerFieldSk>('picker-field-sk')[1];

    // Select multiple items to enable split
    configField.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: ['8888', 'gms'] } })
    );
    await new Promise((resolve) => setTimeout(resolve, 100));

    const splitBox = configField.querySelector('checkbox-sk#split-by') as any;

    let splitEvent: CustomEvent | null = null;
    element.addEventListener('split-by-changed', (e) => {
      splitEvent = e as CustomEvent;
    });

    splitBox.checked = true;
    splitBox.dispatchEvent(new Event('change'));
    await new Promise((resolve) => setTimeout(resolve, 100));

    expect(splitEvent).to.not.be.null;
    expect(splitEvent!.detail.param).to.equal('config');
    expect(splitEvent!.detail.split).to.be.true;
  });
});
