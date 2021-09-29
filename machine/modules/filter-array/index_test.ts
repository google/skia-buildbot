import './index';
import { assert } from 'chai';
import { FilterArray } from './index';

const emulateKeyboardInput = (element: HTMLInputElement, value: string) => {
  element.value = value;
  element.dispatchEvent(new InputEvent('input'));
};

describe('FilterArray', () => {
  let element: HTMLInputElement;
  beforeEach(() => {
    element = document.createElement('input');
  });

  it('returns an empty array from matchingIndices updateArray is called', () => {
    const f = new FilterArray();
    f.connect(element);
    assert.deepEqual(f.matchingValues(), []);
  });

  it('calls callback when input event is triggered', () => {
    let cbCalled = false;
    const f = new FilterArray();
    f.connect(element, () => {
      cbCalled = true;
    });
    emulateKeyboardInput(element, 'foo');
    assert.isTrue(cbCalled);
  });

  it('returns the correct values on matches', () => {
    const f = new FilterArray();
    f.connect(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'foo');
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
  });

  it('matches all elements with an empty filter', () => {
    const f = new FilterArray();
    f.connect(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }, { key: 'bar' }]);
  });

  it('returns an empty array if there are no matches', () => {
    const f = new FilterArray();
    f.connect(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'this string does not appear in any objects');
    assert.deepEqual(f.matchingValues(), []);
  });

  it('returns updated matchingValues after updateArray is called', () => {
    const f = new FilterArray();
    f.connect(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'f');
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
    f.updateArray([{ key: 'foo' }, { key: 'far' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }, { key: 'far' }]);
  });

  it('returns an unfiltered array if things are added to it but no filtration UI is yet connected', () => {
    const f = new FilterArray();
    f.updateArray([{ key: 'foo' }]);
    assert.deepEqual(f.matchingValues(), [{ key: 'foo' }]);
  });

  it('filters the array even when the array is updated before the filtration UI is connected', () => {
    const f = new FilterArray();
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    f.connect(element);
    emulateKeyboardInput(element, 'b');
    assert.deepEqual(f.matchingValues(), [{ key: 'bar' }]);
  });
});
