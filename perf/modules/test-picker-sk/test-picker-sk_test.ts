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
    fetchMock = async (_url: RequestInfo | URL, request: RequestInit | undefined) => {
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
      return await Promise.resolve(new Response(JSON.stringify(response)));
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

  it('should emit remove-explore event when count exceeds limit and graph is present', async () => {
    // Mock the presence of a graph in the container.
    const graphDiv = document.createElement('div');
    graphDiv.id = 'graphContainer';
    graphDiv.appendChild(document.createElement('div')); // Add a child to simulate a loaded graph.
    document.body.appendChild(graphDiv);

    // Re-initialize element to pick up the graphDiv.
    element = newInstance((_el: TestPickerSk) => {});
    element.initializeTestPicker(['benchmark'], {}, false);
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Listen for the event.
    let eventCaught = false;
    element.addEventListener('remove-explore', () => {
      eventCaught = true;
    });

    // Manually trigger updateCount with a large number.
    (element as any).updateCount(201);

    expect(eventCaught).to.be.true;

    // Cleanup
    document.body.removeChild(graphDiv);
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
    window.fetch = async (_url: RequestInfo | URL, request: RequestInit | undefined) => {
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
      return await Promise.resolve(new Response(JSON.stringify(response)));
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
