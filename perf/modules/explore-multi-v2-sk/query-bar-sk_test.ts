import './query-bar-sk';
import { QueryBarSk } from './query-bar-sk';
import { expect } from 'chai';

describe('query-bar-sk', () => {
  let element: QueryBarSk;

  beforeEach(async () => {
    element = document.createElement('query-bar-sk') as QueryBarSk;
    document.body.appendChild(element);
    await element.updateComplete;
  });

  afterEach(() => {
    document.body.removeChild(element);
  });

  it('uses material web components for the query input', () => {
    const input = element.shadowRoot!.querySelectorAll('md-outlined-text-field');
    expect(input.length).to.be.greaterThan(0);
  });

  it('uses Material Design variables for suggestions dropdown', async () => {
    (element as any)['_isOpen'] = true;
    (element as any)['_suggestions'] = [
      { score: 1, params: [{ key: 'bot', value: 'linux' }], count: 42 },
    ];
    await element.updateComplete;

    const dropdown = element.shadowRoot!.querySelector('.suggestions-dropdown') as HTMLElement;
    expect(dropdown).to.not.be.null;

    const item = element.shadowRoot!.querySelector('.suggestion-item') as HTMLElement;
    expect(item).to.not.be.null;

    element.style.setProperty('--on-surface', 'rgb(32, 33, 36)');
    await element.updateComplete;

    const style = window.getComputedStyle(item);
    expect(style.color).to.equal('rgb(32, 33, 36)');

    // Test counter color
    const count = element.shadowRoot!.querySelector('.s-count') as HTMLElement;
    expect(count).to.not.be.null;

    element.style.setProperty('--md-sys-color-on-surface-variant', 'rgb(112, 117, 122)');
    await element.updateComplete;

    const countStyle = window.getComputedStyle(count);
    expect(countStyle.color).to.equal('rgb(112, 117, 122)');
  });

  it('shows categories when focused and empty', async () => {
    element.optionsByKey = {
      benchmark: [{ value: 'blink_perf', count: 10 }],
      device: [{ value: 'm1', count: 5 }],
    };
    element.availableParams = [
      { key: 'benchmark', value: 'blink_perf', count: 10 },
      { key: 'device', value: 'm1', count: 5 },
    ];

    // Simulate focus and empty input
    (element as any)['_isOpen'] = true;
    (element as any)['_inputValue'] = '';
    (element as any)['_updateSuggestions']();

    await element.updateComplete;

    expect((element as any)['_suggestions'].length).to.be.greaterThan(0);
    expect((element as any)['_suggestions'][0].params[0].key).to.equal('benchmark');
  });

  it('shows values when category is selected', async () => {
    element.optionsByKey = {
      benchmark: [{ value: 'blink_perf', count: 10 }],
    };

    (element as any)['_selectedCategory'] = 'benchmark';
    (element as any)['_isOpen'] = true;
    (element as any)['_updateSuggestions']();

    await element.updateComplete;

    expect((element as any)['_suggestions'].length).to.equal(1);
    expect((element as any)['_suggestions'][0].params[0].value).to.equal('blink_perf');
  });

  it('resets category when input is cleared', async () => {
    (element as any)['_selectedCategory'] = 'benchmark';

    (element as any)['_inputValue'] = '';
    (element as any)['_handleInputChange']({ target: { value: '' } } as any);

    expect((element as any)['_selectedCategory']).to.be.null;
  });

  it('sorts categories by includeParams order', async () => {
    element.optionsByKey = {
      benchmark: [{ value: 'blink_perf', count: 10 }],
      device: [{ value: 'm1', count: 5 }],
      arch: [{ value: 'x86', count: 2 }],
    };
    element.includeParams = ['device', 'arch', 'benchmark'];

    (element as any)['_isOpen'] = true;
    (element as any)['_inputValue'] = '';
    (element as any)['_updateSuggestions']();

    await element.updateComplete;

    const suggestions = (element as any)['_suggestions'];
    expect(suggestions.length).to.equal(3);
    expect(suggestions[0].params[0].key).to.equal('device');
    expect(suggestions[1].params[0].key).to.equal('arch');
    expect(suggestions[2].params[0].key).to.equal('benchmark');
  });

  it('boosts priority scores directly', () => {
    element.defaults = {
      default_trigger_priority: {
        metric: ['timeNs'],
      },
    };
    const matches = [
      { p: { key: 'metric', value: 'other' }, score: 10 },
      { p: { key: 'metric', value: 'timeNs' }, score: 10 },
    ];
    (element as any)._boostPriorityScores(matches);
    expect(matches[1].score).to.equal(1010);
    expect(matches[0].score).to.equal(10);
  });

  it('does not open suggestions when clicking on multi-select', async () => {
    element.query = { bot: ['linux'] };
    element.optionsByKey = { bot: [{ value: 'linux', count: 1 }] };
    await element.updateComplete;

    const multiSelect = element.shadowRoot!.querySelector('explore-multi-v2-select-sk');
    expect(multiSelect).to.not.be.null;

    (element as any)['_isOpen'] = false;

    // Simulate click on multi-select
    multiSelect!.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    await element.updateComplete;

    expect((element as any)['_isOpen']).to.be.false;
  });

  it('closes suggestions when opening multi-select', async () => {
    element.query = { bot: ['linux'] };
    element.optionsByKey = { bot: [{ value: 'linux', count: 1 }] };
    await element.updateComplete;

    const multiSelect = element.shadowRoot!.querySelector('explore-multi-v2-select-sk');
    expect(multiSelect).to.not.be.null;

    (element as any)['_isOpen'] = true;

    // Simulate open event from multi-select
    multiSelect!.dispatchEvent(new CustomEvent('open', { bubbles: true }));

    await element.updateComplete;

    expect((element as any)['_isOpen']).to.be.false;
  });
  it('forwards diff-base event and stops propagation', async () => {
    element.query = { bot: ['linux'] };
    element.optionsByKey = { bot: [{ value: 'linux', count: 1 }] };
    await element.updateComplete;

    const multiSelect = element.shadowRoot!.querySelector('explore-multi-v2-select-sk');
    expect(multiSelect).to.not.be.null;

    let eventDetail: any = null;
    let eventCount = 0;
    element.addEventListener('diff-base', (e: any) => {
      eventDetail = e.detail;
      eventCount++;
    });

    // Simulate diff-base event from multi-select
    multiSelect!.dispatchEvent(
      new CustomEvent('diff-base', {
        detail: { key: 'bot', value: 'linux' },
        bubbles: true,
        composed: true,
      })
    );

    expect(eventCount).to.equal(1);
    expect(eventDetail).to.deep.equal({ key: 'bot', value: 'linux' });
  });
});
