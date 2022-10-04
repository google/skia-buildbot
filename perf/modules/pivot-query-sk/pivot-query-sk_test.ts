import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { PivotQueryChangedEventDetail, PivotQueryChangedEventName, PivotQuerySk } from './pivot-query-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ParamSet, pivot } from '../json/all';

describe('pivot-query-sk', () => {
  const newInstance = setUpElementUnderTest<PivotQuerySk>('pivot-query-sk');

  let element: PivotQuerySk;
  beforeEach(() => {
    element = newInstance((el: PivotQuerySk) => {
      const validPivotRequest: pivot.Request = {
        group_by: ['config', 'os'],
        operation: 'avg',
        summary: [],
      };

      const paramSet: ParamSet = {
        config: ['8888', '565'],
        arch: ['x86', 'risc-v'],
        model: ['Pixel2', 'Pixel3'],
      };

      el.pivotRequest = validPivotRequest;
      el.paramset = paramSet;
    });
  });

  describe('click group_by option', () => {
    it('emits event with group_by option added', async () => {
      const ep = eventPromise<CustomEvent<PivotQueryChangedEventDetail>>(PivotQueryChangedEventName);
      // Click 'arch' which will be first, but isn't in the pivot.Request yet.
      $$<HTMLDivElement>('#group_by div', element)!.click();
      const e = await ep;
      assert.isTrue(e.detail!.group_by!.includes('arch'));
    });
  });

  describe('click summary option', () => {
    it('emits event with summary option added', async () => {
      const ep = eventPromise<CustomEvent<PivotQueryChangedEventDetail>>(PivotQueryChangedEventName);
      // Click 'avg' which will be first, but isn't in the pivot.Request yet.
      $$<HTMLDivElement>('#summary div', element)!.click();
      const e = await ep;
      assert.isTrue(e.detail!.summary!.includes('avg'));
    });
  });
});
