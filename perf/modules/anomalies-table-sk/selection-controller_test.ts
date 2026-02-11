import { SelectionController } from './selection-controller';
import { ReactiveControllerHost } from 'lit';
import { assert } from 'chai';

describe('SelectionController', () => {
  let host: ReactiveControllerHost;
  let controller: SelectionController<string>;
  let requestUpdateCalled: boolean;

  beforeEach(() => {
    requestUpdateCalled = false;
    host = {
      addController: () => {},
      removeController: () => {},
      requestUpdate: () => {
        requestUpdateCalled = true;
      },
      updateComplete: Promise.resolve(true),
    } as unknown as ReactiveControllerHost;
    controller = new SelectionController(host);
  });

  it('initially has empty selection', () => {
    assert.equal(controller.size, 0);
    assert.deepEqual(controller.items, []);
  });

  it('selects an item', () => {
    controller.select('a');
    assert.isTrue(controller.has('a'));
    assert.equal(controller.size, 1);
    assert.isTrue(requestUpdateCalled);
  });

  it('deselects an item', () => {
    controller.select('a');
    requestUpdateCalled = false;
    controller.deselect('a');
    assert.isFalse(controller.has('a'));
    assert.equal(controller.size, 0);
    assert.isTrue(requestUpdateCalled);
  });

  it('toggles an item on', () => {
    controller.toggle('a');
    assert.isTrue(controller.has('a'));
    assert.isTrue(requestUpdateCalled);
  });

  it('toggles an item off', () => {
    controller.select('a');
    requestUpdateCalled = false;
    controller.toggle('a');
    assert.isFalse(controller.has('a'));
    assert.isTrue(requestUpdateCalled);
  });

  it('toggles with explicit checked state', () => {
    controller.toggle('a', true);
    assert.isTrue(controller.has('a'));

    controller.toggle('a', true); // Should stay selected
    assert.isTrue(controller.has('a'));

    controller.toggle('a', false);
    assert.isFalse(controller.has('a'));
  });

  it('clears selection', () => {
    controller.select('a');
    controller.select('b');
    requestUpdateCalled = false;
    controller.clear();
    assert.equal(controller.size, 0);
    assert.isTrue(requestUpdateCalled);
  });
});
