/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DataFrame, pivot, TraceSet } from '../json/all';
import {
  KeyValues,
  keyValuesFromTraceSet, PivotTableSk, PivotTableSkChangeEventDetail, SortHistory, SortSelection,
} from './pivot-table-sk';

const df: DataFrame = {
  header: [],
  paramset: {},
  traceset: {
    ',arch=x86,config=8888,': [1, 1.3e27],
    ',arch=arm,config=8888,': [2, 2.3e27],
    ',arch=x86,config=gpu,': [3, 1.2345],
    ',arch=arm,config=gpu,': [3, Math.PI],
  },
  skip: 0,
};

const req: pivot.Request = {
  group_by: ['config', 'arch'],
  operation: 'avg',
  summary: ['avg', 'sum'],
};

const query = 'config=8888&config=gpu&arch=x86&arch=arm';

describe('pivot-table-sk', () => {
  const newInstance = setUpElementUnderTest<PivotTableSk>('pivot-table-sk');

  let element: PivotTableSk;
  beforeEach(() => {
    element = newInstance((el: PivotTableSk) => {
      el.set(df, req, query);
    });
  });

  describe('click sort icon on first column', () => {
    it('sorts column descending', async () => {
      let firstSortSelection = element['sortHistory']!.history[0];
      assert.deepEqual(firstSortSelection, new SortSelection(0, 'summaryValues', 'up'));

      const event = eventPromise<CustomEvent<PivotTableSkChangeEventDetail>>('change');

      // Click on the sort up icon that appears over the 'config' column.
      $$<HTMLElement>('sort-icon-sk', element)!.click();

      const encodedHistory = (await event).detail;

      // Confirm it changes to a drop down icon.
      assert.isNotNull($$<HTMLElement>('arrow-drop-down-icon-sk', element));

      // Confirm sort state has changed.
      firstSortSelection = element['sortHistory']!.history[0];
      assert.deepEqual(firstSortSelection, new SortSelection(0, 'keyValues', 'down'));
      assert.isTrue(encodedHistory.startsWith(firstSortSelection.encode()));
    });
  });
});

describe('SortSelection', () => {
  it('changes the direction on toggleDirection', () => {
    const s = new SortSelection(1, 'summaryValues', 'up');
    s.toggleDirection();
    assert.equal(s.dir, 'down');
  });

  it('builds an accurate compareFunction for summary values', () => {
    const keyValues = keyValuesFromTraceSet(df.traceset, req);
    // Sort up on the second column of summary values, aka 'sum'.
    const s = new SortSelection(1, 'summaryValues', 'up');
    const compare = s.buildCompare(df.traceset, keyValues);

    assert.equal(compare(',arch=x86,config=8888,', ',arch=x86,config=8888,'), 0, 'matching keys returns 0');
    assert.isTrue(compare(',arch=x86,config=8888,', ',arch=arm,config=8888,') < 0, '1.3e27 < 2.3e27 sorting up');
    s.toggleDirection();
    assert.isTrue(compare(',arch=x86,config=8888,', ',arch=arm,config=8888,') > 0, '1.3e27 < 2.3e27 sorting down');
  });

  it('builds a compareFunction that operates correctly in sort for summary values', () => {
    const keyValues = keyValuesFromTraceSet(df.traceset, req);
    // Sort up on the second column of summary values, aka 'sum'.
    const s = new SortSelection(1, 'summaryValues', 'up');
    const compare = s.buildCompare(df.traceset, keyValues);

    assert.deepEqual(
      Object.keys(df.traceset).sort(compare).map((traceKey) => df.traceset[traceKey][1]),
      [
        1.2345,
        Math.PI,
        1.3e27,
        2.3e27,
      ],
    );
  });

  it('builds an accurate compareFunction for key values', () => {
    const keyValues = keyValuesFromTraceSet(df.traceset, req);
    // Sort up on the first column of key values, aka 'config'.
    const s = new SortSelection(0, 'keyValues', 'up');
    const compare = s.buildCompare(df.traceset, keyValues);

    assert.equal(compare(',arch=x86,config=gpu,', ',arch=x86,config=gpu,'), 0, 'matching keys returns 0');
    assert.equal(compare(',arch=arm,config=8888,', ',arch=x86,config=gpu,'), -1, '8888 < gpu sorting up');
    s.toggleDirection();
    assert.equal(compare(',arch=arm,config=8888,', ',arch=x86,config=gpu,'), 1, '8888 < gpu sorting down');
  });

  it('round trips through encode and decode', () => {
    const expected = new SortSelection(2, 'keyValues', 'down');
    const encoded = expected.encode();
    assert.equal(encoded, 'dk2');
    const actual = SortSelection.decode(expected.encode());
    assert.deepEqual(actual, expected);
  });

  it('decode robustly handles invalid strings', () => {
    const actual = SortSelection.decode('');
    assert.deepEqual(actual, new SortSelection(0, 'summaryValues', 'down'));
  });
});

describe('SortHistory', () => {
  it('moves selected columns to the front of the list and toggles their direction', () => {
    const history = new SortHistory(req.group_by!.length, req.summary!.length);

    const first = history.history[0];
    assert.deepEqual(first, new SortSelection(0, 'summaryValues', 'up'));

    // If we select the second keyValues column to sort on then it should move to
    // the front of the history list and be in the opposite direction.

    history.selectColumnToSortOn(1, 'keyValues');

    const newFirst = history.history[0];
    assert.deepEqual(newFirst, new SortSelection(1, 'keyValues', 'down'));
  });

  it('builds a compareFunction that operates correctly in sort', () => {
    const history = new SortHistory(req.group_by!.length, req.summary!.length);

    // Configure history so that is sorts on the second key value down, and
    // breaks ties by looking at the first key value also in the down direction:
    history.selectColumnToSortOn(0, 'summaryValues');
    history.selectColumnToSortOn(1, 'summaryValues');

    // Sort this traceset and it should come out in this order:
    const traceset: TraceSet = {
      ',arch=x86,config=gpu,': [4, 2],
      ',arch=arm,config=gpu,': [3, 2],
      ',arch=arm,config=8888,': [2, 1],
      ',arch=x86,config=8888,': [1, 1],
    };
    const keyValues = keyValuesFromTraceSet(traceset, req);
    const keys = Object.keys(traceset);
    keys.sort(history.buildCompare(traceset, keyValues));
    const expected = [
      ',arch=x86,config=gpu,',
      ',arch=arm,config=gpu,',
      ',arch=arm,config=8888,',
      ',arch=x86,config=8888,',
    ];
    assert.deepEqual(keys, expected);
  });
});

describe('keyValuesFromTraceSet', () => {
  it('returns empty keyValues for an empty TraceSet', () => {
    assert.isEmpty(keyValuesFromTraceSet({}, req));
  });

  it('correctly orders key values based on the request', () => {
    const actual = keyValuesFromTraceSet(df.traceset, req);
    const expected: KeyValues = {
      ',arch=x86,config=gpu,': ['gpu', 'x86'],
      ',arch=arm,config=gpu,': ['gpu', 'arm'],
      ',arch=arm,config=8888,': ['8888', 'arm'],
      ',arch=x86,config=8888,': ['8888', 'x86'],
    };
    assert.deepEqual(actual, expected);
  });

  it('drops keys that do not appear in the pivot.Request.group_by', () => {
    const reqWithOnlyOneGroupBy: pivot.Request = {
      group_by: ['arch'], // Only has arch.
      operation: 'avg',
      summary: ['avg', 'sum'],
    };
    const actual = keyValuesFromTraceSet(df.traceset, reqWithOnlyOneGroupBy);
    const expected: KeyValues = {
      ',arch=x86,config=gpu,': ['x86'],
      ',arch=arm,config=gpu,': ['arm'],
      ',arch=arm,config=8888,': ['arm'],
      ',arch=x86,config=8888,': ['x86'],
    };
    assert.deepEqual(actual, expected);
  });

  it('round trips through encode and decode', () => {
    const expected = new SortHistory(req.group_by!.length, req.summary!.length);
    const actual = new SortHistory(req.group_by!.length, req.summary!.length);
    actual.history = [];
    actual.decode(expected.encode());
    assert.deepEqual(actual, expected);
  });
});
