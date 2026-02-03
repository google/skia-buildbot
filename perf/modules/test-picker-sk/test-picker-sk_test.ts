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
