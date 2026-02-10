import './index';
import { assert } from 'chai';
import { ToastSk } from './toast-sk';
import * as sinon from 'sinon';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('toast-sk', () => {
  const newInstance = setUpElementUnderTest<ToastSk>('toast-sk');

  let element: ToastSk;

  beforeEach(() => {
    element = newInstance((el) => {
      el.innerHTML = '<span>Hello World</span>';
    });
  });

  it('is defined', () => {
    assert.isDefined(element);
  });

  it('renders children (preserves Light DOM)', async () => {
    assert.include(element.innerHTML, '<span>Hello World</span>');
    await element.updateComplete;
    assert.include(element.innerHTML, '<span>Hello World</span>');
  });

  it('reflects duration property', async () => {
    element.duration = 3000;
    await element.updateComplete;
    assert.equal(element.getAttribute('duration'), '3000');
  });

  it('shows and hides', async () => {
    assert.isFalse(element.shown);
    assert.isFalse(element.hasAttribute('shown'));

    element.show();
    await element.updateComplete;
    assert.isTrue(element.shown);
    assert.isTrue(element.hasAttribute('shown'));

    element.hide();
    await element.updateComplete;
    assert.isFalse(element.shown);
    assert.isFalse(element.hasAttribute('shown'));
  });

  it('hides automatically after duration', async () => {
    // Mock timer
    const clock = sinon.useFakeTimers();
    try {
      element.duration = 100;
      element.show();
      await element.updateComplete;
      assert.isTrue(element.shown);

      clock.tick(150);
      await element.updateComplete;
      assert.isFalse(element.shown);
    } finally {
      clock.restore();
    }
  });
});
