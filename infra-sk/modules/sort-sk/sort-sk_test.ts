import './index';
import { $ } from 'common-sk/modules/dom';

import { assert } from 'chai';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('sort-sk', () => {
  it('sorts numerically by default', () => window.customElements.whenDefined('sort-sk').then(() => {
    container.innerHTML = `
  <sort-sk target=stuffToBeSorted>
    <button id=cluster data-key=clustersize data-default=up>Cluster Size</button>
    <button id=size data-key=stepsize>Step Size</button>
  </sort-sk>

  <div id=stuffToBeSorted>
    <pre data-clustersize=10  data-stepsize=1.2>Size=10   Step=1.2</pre>
    <pre data-clustersize=50  data-stepsize=0.5>Size=50   Step=0.5</pre>
    <pre data-clustersize=100 data-stepsize=0.6>Size=100  Step=0.6</pre>
  </div>`;
    const getValues = (name: string) => $<HTMLPreElement>('#stuffToBeSorted pre', container).map(
      (ele) => +(ele.dataset[name] || ''),
    );

    const clusterButton = container.querySelector<HTMLButtonElement>(
      '#cluster',
    )!;
    const stepButton = container.querySelector<HTMLButtonElement>('#size')!;

    clusterButton.click();
    assert.deepEqual(
      [100, 50, 10],
      getValues('clustersize'),
      'Defaults to up, so sort down on first click.',
    );
    clusterButton.click();
    assert.deepEqual(
      [10, 50, 100],
      getValues('clustersize'),
      'Switch to up.',
    );

    stepButton.click();
    assert.deepEqual(
      [1.2, 0.6, 0.5],
      getValues('stepsize'),
      'No default, so start sorting down.',
    );
    stepButton.click();
    assert.deepEqual([0.5, 0.6, 1.2], getValues('stepsize'), 'Switch to up.');
  }));

  it('sorts alphabetically with alpha attribute', () => window.customElements.whenDefined('sort-sk').then(() => {
    container.innerHTML = `
  <sort-sk target=stuffToBeSorted2>
    <button id=name  data-key=name data-default=down data-sort-type=alpha>Name</button>
    <button id=level data-key=level                  data-sort-type=alpha>Level</button>
  </sort-sk>

  <div id=stuffToBeSorted2>
    <pre data-name=foo data-level=alpha>foo alpha</pre>
    <pre data-name=baz data-level=beta >baz beta</pre>
    <pre data-name=bar data-level=gamma>bar gamma</pre>
  </div>
  `;
    const getValues = (name: string) => $<HTMLPreElement>('#stuffToBeSorted2 pre', container).map(
      (ele) => ele.dataset[name],
    );

    const nameButton = container.querySelector<HTMLButtonElement>('#name')!;
    const levelButton = container.querySelector<HTMLButtonElement>('#level')!;

    nameButton.click();
    assert.deepEqual(
      ['bar', 'baz', 'foo'],
      getValues('name'),
      'Defaults to down, so sort up.',
    );
    nameButton.click();
    assert.deepEqual(
      ['foo', 'baz', 'bar'],
      getValues('name'),
      'Now switch to down.',
    );

    levelButton.click();
    assert.deepEqual(
      ['gamma', 'beta', 'alpha'],
      getValues('level'),
      'No default, so sort down.',
    );
    levelButton.click();
    assert.deepEqual(
      ['alpha', 'beta', 'gamma'],
      getValues('level'),
      'Now switch to up.',
    );
  }));
});
