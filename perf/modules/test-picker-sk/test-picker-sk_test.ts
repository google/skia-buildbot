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
    fetchMock = (_url: string, request: any) => {
      const req = JSON.parse(request.body) as NextParamListHandlerRequest;
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
