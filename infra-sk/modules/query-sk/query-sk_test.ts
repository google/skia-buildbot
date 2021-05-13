import './index';
import { ParamSet, toParamSet, fromParamSet } from 'common-sk/modules/query';
import { removePrefix, QuerySk } from './query-sk';
import { QuerySkPO } from './query-sk_po';
import { setUpElementUnderTest } from '../test_util';
import { assert } from 'chai';

const paramset: ParamSet = {
  arch: ['WASM', 'arm', 'arm64', 'asmjs', 'wasm', 'x86', 'x86_64'],
  bench_type: [
    'deserial',
    'micro',
    'playback',
    'recording',
    'skandroidcodec',
    'skcodec',
    'tracing',
  ],
  compiler: ['Clang', 'EMCC', 'GCC'],
  config: ['8888', 'f16', 'gl', 'gles'],
};

describe('query-sk', () => {
  const newInstance = setUpElementUnderTest<QuerySk>('query-sk');

  let querySk: QuerySk;
  let querySkPO: QuerySkPO;

  beforeEach(() => {
    querySk = newInstance();
    querySk.paramset = paramset;
    querySkPO = new QuerySkPO(querySk);
  });

  it('sets the available options via the "paramset" property', async () => {
    assert.deepEqual(paramset, await querySkPO.getParamSet());
  });

  it('can change the selection on the UI via the current_query property', async () => {
    const query: ParamSet = {
      arch: ['arm', 'x86'],
      config: ['!8888'],
      compiler: ['~CC'],
    };

    querySk.current_query = fromParamSet(query);
    assert.deepEqual(query, await querySkPO.getCurrentQuery());
  });

  it('can change the current_query property via the UI', async () => {
    const query: ParamSet = {
      arch: ['arm', 'x86'],
      config: ['!8888'],
      compiler: ['~CC'],
    };

    await querySkPO.setCurrentQuery(query);
    assert.deepEqual(fromParamSet(query), querySk.current_query);
  });

  it('obeys key_order', async () => {
    assert.deepEqual(
      ['arch', 'bench_type', 'compiler', 'config'],
      await querySkPO.getKeys()
    );

    // Setting key_order will change the key order.
    querySk.key_order = ['config'];
    assert.deepEqual(
      ['config', 'arch', 'bench_type', 'compiler'],
      await querySkPO.getKeys()
    );

    // Setting key_order to empty will go back to alphabetical order.
    querySk.key_order = [];
    assert.deepEqual(
      ['arch', 'bench_type', 'compiler', 'config'],
      await querySkPO.getKeys()
    );
  });

  it('obeys filter', async () => {
    assert.deepEqual(
      ['arch', 'bench_type', 'compiler', 'config'],
      await querySkPO.getKeys()
    );

    // Setting the filter will change the keys displayed.
    await querySkPO.setFilter('cro'); // Only 'micro' in 'bench_type' should match.

    // Only key should be bench_type.
    assert.deepEqual(['bench_type'], await querySkPO.getKeys());

    // Clearing the filter will restore all options.
    await querySkPO.clickClearFilter();

    assert.deepEqual(
      ['arch', 'bench_type', 'compiler', 'config'],
      await querySkPO.getKeys()
    );
  });

  it('only edits displayed values when filter is used.', async () => {
    // Make a selection.
    querySk.current_query = 'arch=x86';

    // Setting the filter will change the keys displayed.
    await querySkPO.setFilter('64'); // Only 'arm64' and 'x86_64' in 'arch' should match.

    // Only key should be arch.
    assert.deepEqual(['arch'], await querySkPO.getKeys());

    // Click on 'arch'.
    await querySkPO.clickKey('arch');

    // Click on the value 'arm64' to add it to the query.
    await querySkPO.clickValue('arm64');

    // Confirm it gets added.
    assert.deepEqual(
      toParamSet('arch=x86&arch=arm64'),
      toParamSet(querySk.current_query)
    );

    // Click on the value 'arm64' a second time to remove it from the query.
    await querySkPO.clickValue('arm64');

    // Confirm it gets removed.
    assert.deepEqual(toParamSet('arch=x86'), toParamSet(querySk.current_query));
  });

  it('only edits displayed inverted values when filter is used.', async () => {
    // Make a selection.
    querySk.current_query = 'arch=!x86';

    // Setting the filter will change the keys displayed.
    await querySkPO.setFilter('64'); // Only 'arm64' and 'x86_64' in 'arch' should match.

    // Only key should be arch.
    assert.deepEqual(['arch'], await querySkPO.getKeys());

    // Click on 'arch'.
    await querySkPO.clickKey('arch');

    // Click on the value 'arm64' to add it to the query.
    await querySkPO.clickValue('arm64');

    // Confirm it gets added.
    assert.deepEqual(
      toParamSet('arch=!x86&arch=!arm64'),
      toParamSet(querySk.current_query)
    );

    // Click on the value 'arm64' a second time to remove it from the query.
    await querySkPO.clickValue('arm64');

    // Confirm it gets removed.
    assert.deepEqual(
      toParamSet('arch=!x86'),
      toParamSet(querySk.current_query)
    );
  });

  it('sets invert correctly on query values even if they are hidden by a filter.', async () => {
    // Make a selection.
    querySk.current_query = 'arch=x86';

    // Setting the filter will change the keys displayed.
    await querySkPO.setFilter('64'); // Only 'arm64' and 'x86_64' in 'arch' should match.

    // Only key should be arch.
    assert.deepEqual(['arch'], await querySkPO.getKeys());

    // Click on 'arch'.
    await querySkPO.clickKey('arch');

    // Click the invert checkbox.
    await (await querySkPO.queryValuesSkPO)?.clickInvertCheckbox();

    // Confirm that the undisplayed query values get inverted.
    assert.equal(querySk.current_query, 'arch=!x86');

    // Now go the other way, back to no-invert.
    await (await querySkPO.queryValuesSkPO)?.clickInvertCheckbox();

    // Confirm that the undisplayed query values get inverted.
    assert.equal(querySk.current_query, 'arch=x86');
  });

  it('clears query values on regex.', async () => {
    // Make a selection.
    querySk.current_query = 'arch=~x';

    // Setting the filter will change the keys displayed.
    await querySkPO.setFilter('64'); // Only 'arm64' and 'x86_64' in 'arch' should match.

    // Only key should be arch.
    assert.deepEqual(['arch'], await querySkPO.getKeys());

    // Click on 'arch'.
    await querySkPO.clickKey('arch');

    // Click the regex checkbox.
    await (await querySkPO.queryValuesSkPO)?.clickRegexCheckbox();

    // Confirm that the current_query gets cleared.
    assert.equal(querySk.current_query, '');
  });

  it('updates query-values-sk when the current_query property is set', async () => {
    // Click on 'arch'.
    await querySkPO.clickKey('arch');

    // Click on the value 'arm' to add it to the query.
    await querySkPO.clickValue('arm');

    // Assert that only 'arm' is selected.
    assert.deepEqual(['arm'], await querySkPO.getSelectedValues());

    // Set selection via current_query.
    querySk.current_query = 'arch=x86&arch=x86_64&config=8888';

    // Assert that the previous selection is reflected in the UI.
    assert.deepEqual(['x86', 'x86_64'], await querySkPO.getSelectedValues());
  });

  it('rationalizes current_query when set programmatically', async () => {
    const validQuery = fromParamSet({
      arch: ['arm', 'x86'],
      config: ['!8888'],
      compiler: ['~CC'],
    });
    const invalidQuery = fromParamSet({
      arch: ['arm', 'invalid_architecture'],
      invalid_key: ['foo'],
    });
    const invalidQueryRationalized = fromParamSet({ arch: ['arm'] });

    // Valid queries should remain unaltered.
    querySk.current_query = validQuery;
    assert.deepEqual(validQuery, querySk.current_query);
    assert.deepEqual(
      validQuery,
      fromParamSet(await querySkPO.getCurrentQuery())
    );

    // Invalid queries should be rationalized.
    querySk.current_query = invalidQuery;
    assert.deepEqual(invalidQueryRationalized, querySk.current_query);
    assert.deepEqual(
      invalidQueryRationalized,
      fromParamSet(await querySkPO.getCurrentQuery())
    );
  });

  it('clears the selection when the "Clear Selections" button is clicked', async () => {
    const query: ParamSet = {
      arch: ['arm', 'x86'],
      config: ['!8888'],
      compiler: ['~CC'],
    };

    querySk.current_query = fromParamSet(query);
    assert.deepEqual(query, await querySkPO.getCurrentQuery());

    await querySkPO.clickClearSelections();
    assert.deepEqual('', querySk.current_query);
    assert.deepEqual({}, await querySkPO.getCurrentQuery());
  });
});

describe('removePrefix', () => {
  it('removes reges prefix', () => {
    assert.equal('foo', removePrefix('~foo'));
  });

  it('removes invert prefix', () => {
    assert.equal('foo', removePrefix('!foo'));
  });

  it('leaves unprefixed values unchanged', () => {
    assert.equal('foo', removePrefix('foo'));
  });
});
