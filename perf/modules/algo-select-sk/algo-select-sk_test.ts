import './index';
import { assert } from 'chai';
import { AlgoSelectSk, AlgoSelectAlgoChangeEventDetail } from './algo-select-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('algo-select-sk', () => {
  const newInstance = setUpElementUnderTest<AlgoSelectSk>('algo-select-sk');

  let element: AlgoSelectSk;
  beforeEach(() => {
    element = newInstance();
  });

  const dispatchSelectionChanged = (selection: number) => {
    const select = element.querySelector('select-sk')!;
    select.dispatchEvent(
      new CustomEvent('selection-changed', {
        detail: { selection },
        bubbles: true,
      })
    );
  };

  const waitForAlgoChange = async () =>
    await new Promise<CustomEvent<AlgoSelectAlgoChangeEventDetail>>((resolve) => {
      element.addEventListener(
        'algo-change',
        (e) => {
          resolve(e as CustomEvent<AlgoSelectAlgoChangeEventDetail>);
        },
        { once: true }
      );
    });

  it('defaults to kmeans', () => {
    assert.equal(element.algo, 'kmeans');
  });

  it('can set algo through attribute', () => {
    element.setAttribute('algo', 'stepfit');
    assert.equal(element.algo, 'stepfit');
  });

  it('can set algo through property', () => {
    element.algo = 'stepfit';
    assert.equal(element.getAttribute('algo'), 'stepfit');
    assert.equal(element.algo, 'stepfit');
  });

  it('falls back to kmeans for invalid values', () => {
    element.setAttribute('algo', 'invalid');
    assert.equal(element.algo, 'kmeans');
  });

  it('emits algo-change event when selection changes', async () => {
    const eventPromise = waitForAlgoChange();

    // select-sk selection changed is index based.
    // 0 is K-Means, 1 is Individual (stepfit)
    dispatchSelectionChanged(1);

    const event = await eventPromise;
    assert.equal(event.detail.algo, 'stepfit');
  });

  it('handles negative selection in select-sk', async () => {
    const eventPromise = waitForAlgoChange();

    dispatchSelectionChanged(-1);

    const event = await eventPromise;
    assert.equal(event.detail.algo, 'kmeans');
  });
});
