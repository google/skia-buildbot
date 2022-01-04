/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DataFrame, pivot } from '../json';
import { PivotTableSk } from './pivot-table-sk';

describe('pivot-table-sk', () => {
  const newInstance = setUpElementUnderTest<PivotTableSk>('pivot-table-sk');

  let element: PivotTableSk;
  beforeEach(() => {
    element = newInstance((el: PivotTableSk) => {
      const df: DataFrame = {
        header: [],
        paramset: {},
        traceset: {
          ',config=8888,': [1.2345, 1.3e27],
          ',config=gpu,': [0.1234, Math.PI],
        },
        skip: 0,
      };

      const req: pivot.Request = {
        group_by: ['config'],
        operation: 'avg',
        summary: ['avg', 'sum'],
      };
      el.set(df, req);
    });
  });

  describe('click sort icon on first column', () => {
    it('sorts column descending', async () => {
      assert.equal(element['sortBy'], 0);
      assert.equal(element['sortDirection'], 'up');
      assert.equal(element['sortKind'], 'summaryValues');

      // Click on the sort up icon that appears over the 'config' column.
      $$<HTMLElement>('sort-icon-sk', element)!.click();

      // Confirm it changes to a drop down icon.
      assert.isNotNull($$<HTMLElement>('arrow-drop-down-icon-sk', element));

      // Confirm sort state has changed.
      assert.equal(element['sortBy'], 0);
      assert.equal(element['sortDirection'], 'down');
      assert.equal(element['sortKind'], 'keyValues');
    });
  });
});
