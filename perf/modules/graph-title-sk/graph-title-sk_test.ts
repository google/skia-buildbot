import './index';
import { assert } from 'chai';
import { GraphTitleSk } from './graph-title-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('graph-title-sk', () => {
  const newInstance = setUpElementUnderTest<GraphTitleSk>('graph-title-sk');

  let element: GraphTitleSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('is hidden when numTraces is 0', async () => {
    element.set(new Map([['a', 'b']]), 0);
    await element.updateComplete;
    const container = element.querySelector('#container')!;
    assert.isTrue(container.hasAttribute('hidden'));
  });

  it('renders "Multi-trace Graph" when titleEntries is empty and numTraces > 0', async () => {
    element.set(new Map(), 5);
    await element.updateComplete;
    const h1 = element.querySelector('h1')!;
    assert.equal(h1.textContent, 'Multi-trace Graph (5 traces)');
  });

  it('renders title entries correctly', async () => {
    element.set(
      new Map([
        ['benchmark', 'Speedometer2'],
        ['bot', 'linux-perf'],
      ]),
      1
    );
    await element.updateComplete;
    const params = element.querySelectorAll('.param');
    const values = element.querySelectorAll('.hover-to-show-text');

    assert.equal(params.length, 2);
    assert.equal(params[0].textContent, 'benchmark');
    assert.equal(values[0].textContent, 'Speedometer2');
    assert.equal(params[1].textContent, 'bot');
    assert.equal(values[1].textContent, 'linux-perf');
  });

  it('ignores empty keys or values', async () => {
    element.set(
      new Map([
        ['benchmark', ''],
        ['', 'linux-perf'],
        ['test', 'valid'],
      ]),
      1
    );
    await element.updateComplete;
    const params = element.querySelectorAll('.param');
    assert.equal(params.length, 1);
    assert.equal(params[0].textContent, 'test');
  });

  it('shows short title and "Show Full Title" button when many entries', async () => {
    const entries = new Map<string, string>();
    for (let i = 0; i < 10; i++) {
      entries.set(`key${i}`, `value${i}`);
    }
    element.set(entries, 1);
    await element.updateComplete;

    // MAX_PARAMS is 8.
    const columns = element.querySelectorAll('.column');
    assert.equal(columns.length, 8);

    const button = element.querySelector('md-text-button.showMore')!;
    assert.isNotNull(button);
  });

  it('shows full title when clicking "Show Full Title"', async () => {
    const entries = new Map<string, string>();
    for (let i = 0; i < 10; i++) {
      entries.set(`key${i}`, `value${i}`);
    }
    element.set(entries, 1);
    await element.updateComplete;

    const button = element.querySelector<HTMLElement>('md-text-button.showMore')!;
    button.click();
    await element.updateComplete;

    const columns = element.querySelectorAll('.column');
    assert.equal(columns.length, 10);
    assert.isNull(element.querySelector('md-text-button.showMore'));
  });

  it('can toggle back to short titles', async () => {
    const entries = new Map<string, string>();
    for (let i = 0; i < 10; i++) {
      entries.set(`key${i}`, `value${i}`);
    }
    element.set(entries, 1);
    await element.updateComplete;

    element.showFullTitle();
    await element.updateComplete;
    assert.equal(element.querySelectorAll('.column').length, 10);

    element.showShortTitles();
    await element.updateComplete;
    assert.equal(element.querySelectorAll('.column').length, 8);
    assert.isNotNull(element.querySelector('md-text-button.showMore'));
  });
});
