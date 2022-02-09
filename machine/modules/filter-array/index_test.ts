import './index';
import { assert } from 'chai';
import { FilterArray } from './index';

describe('FilterArray', () => {
  let element: HTMLInputElement;
  beforeEach(() => {
    element = document.createElement('input');
  });

  it('returns an empty array from matchingValues updateArray is called', () => {
    const f = new FilterArray();
    assert.deepEqual(f.matchingValues(), []);
  });

  it('returns the correct values on matches', () => {
    const f = new FilterArray();
    f.filterChanged('foo');
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
  });

  it('returns the AND of all the filter words', () => {
    const f = new FilterArray();
    f.filterChanged('foo bar');
    f.updateArray([{ key: 'foo' }, { key: 'bar' }, { key: 'foo-bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo-bar' }]);
  });

  it('matches all elements with an empty filter', () => {
    const f = new FilterArray();
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }, { key: 'bar' }]);
  });

  it('matches all elements with a non-empty filter that is all spaces', () => {
    const f = new FilterArray();
    f.filterChanged('   ');
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }, { key: 'bar' }]);
  });

  it('returns an empty array if there are no matches', () => {
    const f = new FilterArray();
    f.filterChanged('this string does not appear in any objects');
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), []);
  });

  it('returns updated matchingValues after updateArray is called', () => {
    const f = new FilterArray();
    f.filterChanged('f');
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
    f.updateArray([{ key: 'foo' }, { key: 'far' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }, { key: 'far' }]);
  });

  it('returns an unfiltered array if things are added to it but filterChanged has never been called', () => {
    const f = new FilterArray();
    f.updateArray([{ key: 'foo' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
  });

  it('filters the array even when the array is updated before is filterChanged called', () => {
    const f = new FilterArray();
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    f.filterChanged('b');
    assert.deepEqual(f.matchingValues(), [{ key: 'bar' }]);
  });
});
