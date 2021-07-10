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
    const f = new FilterArray(element);
    assert.deepEqual(f.matchingIndices(), []);
  });

  it('calls callback when input event is triggered', () => {
    let cbCalled = false;
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const f = new FilterArray(element, () => {
      cbCalled = true;
    });
    emulateKeyboardInput(element, 'foo');
    assert.isTrue(cbCalled);
  });

  it('returns the correct index on matches', () => {
    const f = new FilterArray(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'foo');
    assert.deepEqual(f.matchingIndices(), [0]);
  });

  it('matches all elements with an empty filter', () => {
    const f = new FilterArray(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    assert.deepEqual(f.matchingIndices(), [0, 1]);
  });

  it('returns an empty array if there are no matches', () => {
    const f = new FilterArray(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'this string does not appear in any objects');
    assert.deepEqual(f.matchingIndices(), []);
  });

  it('returns updated matchingIndices after updateValue is called', () => {
    const f = new FilterArray(element);
    f.updateArray([{ key: 'foo' }, { key: 'bar' }]);
    emulateKeyboardInput(element, 'f');
    assert.deepEqual(f.matchingIndices(), [0]);
    f.updateArray([{ key: 'foo' }, { key: 'far' }]);
    assert.deepEqual(f.matchingIndices(), [0, 1]);
  });
});
