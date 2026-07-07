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

  it('dispatches pipeline-change event when formula step is added', async () => {
    let detail: any = null;
    element.addEventListener('pipeline-change', (e: any) => {
      detail = e.detail;
    });

    const addBtn = element.shadowRoot!.querySelector('.qb-add-formula-btn') as HTMLButtonElement;
    expect(addBtn).to.not.be.null;
    addBtn.click();
    await element.updateComplete;

    const item = element.shadowRoot!.querySelector('.qb-formula-item') as HTMLButtonElement;
    expect(item).to.not.be.null;
    item.click();

    expect(detail).to.deep.equal({ pipeline: ['fill'] });
  });

  it('renders pipeline arrows between steps but not trailing', async () => {
    element.pipeline = ['fill', 'ave'];
    await element.updateComplete;

    const arrows = element.shadowRoot!.querySelectorAll('.qb-pipeline-arrow');
    expect(arrows.length).to.equal(1); // 2 steps -> 1 arrow between them
  });

  it('renders no pipeline arrows for single step', async () => {
    element.pipeline = ['fill'];
    await element.updateComplete;

    const arrows = element.shadowRoot!.querySelectorAll('.qb-pipeline-arrow');
    expect(arrows.length).to.equal(0);
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

  it('dispatches clone-query event when clone button is clicked', async () => {
    let cloneClicked = false;
    element.addEventListener('clone-query', () => {
      cloneClicked = true;
    });

    await element.updateComplete;
    const cloneBtn = element.shadowRoot!.querySelector('.qb-clone-query-btn') as HTMLElement;
    expect(cloneBtn).to.not.be.null;

    cloneBtn.click();
    expect(cloneClicked).to.be.true;
  });

  it('grays out counts when loading is true', async () => {
    (element as any)['_isOpen'] = true;
    (element as any)['_suggestions'] = [
      { score: 1, params: [{ key: 'bot', value: 'linux' }], count: 42 },
    ];
    element.loading = true;
    await element.updateComplete;

    const count = element.shadowRoot!.querySelector('.s-count') as HTMLElement;
    expect(count).to.not.be.null;
    expect(count.classList.contains('stale')).to.be.true;

    element.loading = false;
    await element.updateComplete;
    expect(count.classList.contains('stale')).to.be.false;
  });

  it('shows spinner when loading is true', async () => {
    element.loading = true;
    await element.updateComplete;

    const spinner = element.shadowRoot!.querySelector('.input-spinner');
    expect(spinner).to.not.be.null;

    element.loading = false;
    await element.updateComplete;
    const spinnerHidden = element.shadowRoot!.querySelector('.input-spinner');
    expect(spinnerHidden).to.be.null;
  });

  describe('pill selection and clipboard actions', () => {
    beforeEach(async () => {
      element.query = { benchmark: ['v8'], bot: ['MacM1'] };
      element.optionsByKey = {
        benchmark: [{ value: 'v8', count: 1 }],
        bot: [{ value: 'MacM1', count: 1 }],
      };
      await element.updateComplete;
    });

    it('toggles selection on Ctrl+Click and does not open dropdown', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      expect(pills.length).to.equal(2);

      const firstPill = pills[0] as any;
      const secondPill = pills[1] as any;

      // Initially no pills are highlighted
      expect(firstPill.isHighlighted).to.be.false;
      expect(secondPill.isHighlighted).to.be.false;

      // Ctrl+Click first pill
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      // First pill should be highlighted, second should not
      expect(firstPill.isHighlighted).to.be.true;
      expect(secondPill.isHighlighted).to.be.false;
      expect((element as any)['_openPillIndex']).to.be.null;

      // Ctrl+Click first pill again to toggle off
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      expect(firstPill.isHighlighted).to.be.false;
    });

    it('clears selection and opens dropdown on normal click', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;

      // Highlight first pill via Ctrl+Click
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;
      expect(firstPill.isHighlighted).to.be.true;

      // Normal click first pill
      firstPill.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      await element.updateComplete;

      // Highlight should be cleared
      expect(firstPill.isHighlighted).to.be.false;
      expect((element as any)['_selectedPills'].size).to.equal(0);
    });

    it('supports Shift+Click range selection', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;
      const secondPill = pills[1] as any;

      // Set anchor by Ctrl+Clicking first pill
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      // Shift+Click second pill
      secondPill.dispatchEvent(new MouseEvent('click', { shiftKey: true, bubbles: true }));
      await element.updateComplete;

      // Both should be highlighted
      expect(firstPill.isHighlighted).to.be.true;
      expect(secondPill.isHighlighted).to.be.true;
    });

    it('copies selected pills to clipboard on Ctrl+C', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;
      const secondPill = pills[1] as any;

      // Select first and second pills
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      secondPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      // Mock clipboard using Object.defineProperty to bypass read-only TS and runtime checks
      let copiedText = '';
      const originalClipboard = navigator.clipboard;
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: async (text: string) => {
            copiedText = text;
          },
        },
        configurable: true,
      });

      // Dispatch Ctrl+C on input (since focus returns to input after Ctrl+Click)
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLElement;
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'c', ctrlKey: true, bubbles: true }));

      expect(copiedText).to.equal('benchmark=v8 bot=MacM1');

      // Restore clipboard
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    });

    it('cuts selected pills to clipboard and deletes them on Ctrl+X', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;

      // Select first pill
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      // Mock clipboard
      let copiedText = '';
      const originalClipboard = navigator.clipboard;
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: async (text: string) => {
            copiedText = text;
          },
        },
        configurable: true,
      });

      let removedKey = '';
      element.addEventListener('remove-key', (e: any) => {
        removedKey = e.detail.key;
      });

      // Dispatch Ctrl+X on input
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLElement;
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', ctrlKey: true, bubbles: true }));

      expect(copiedText).to.equal('benchmark=v8');
      expect(removedKey).to.equal('benchmark');

      // Restore clipboard
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    });

    it('cuts selected pills and text to clipboard and clears them on Ctrl+X', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;

      // Select first pill
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      // Type some text
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLInputElement;
      input.value = 'some text';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await element.updateComplete;

      // Select the text
      input.setSelectionRange(0, 9);
      document.dispatchEvent(new Event('selectionchange', { bubbles: true }));

      // Mock clipboard
      let copiedText = '';
      const originalClipboard = navigator.clipboard;
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: async (text: string) => {
            copiedText = text;
          },
        },
        configurable: true,
      });

      let removedKey = '';
      element.addEventListener('remove-key', (e: any) => {
        removedKey = e.detail.key;
      });

      // Dispatch Ctrl+X on input
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'x', ctrlKey: true, bubbles: true }));

      expect(copiedText).to.equal('benchmark=v8 some text');
      expect(removedKey).to.equal('benchmark');
      expect((element as any)['_inputValue']).to.equal('');

      // Restore clipboard
      Object.defineProperty(navigator, 'clipboard', {
        value: originalClipboard,
        configurable: true,
      });
    });

    it('pastes query string and dispatches add-query events via paste event', async () => {
      const addedQueries: { key: string; value: string }[] = [];
      element.addEventListener('add-query', (e: any) => {
        addedQueries.push({ key: e.detail.key, value: e.detail.value });
      });

      // Create a paste event with custom clipboardData
      const pasteData = 'os=Android device=Pixel6';
      const dataTransfer = new DataTransfer();
      dataTransfer.setData('text', pasteData);

      const pasteEvent = new ClipboardEvent('paste', {
        bubbles: true,
        cancelable: true,
        clipboardData: dataTransfer,
      });

      // Dispatch paste on input
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLElement;
      input.dispatchEvent(pasteEvent);

      await element.updateComplete;

      expect(addedQueries).to.have.lengthOf(2);
      expect(addedQueries[0]).to.deep.equal({ key: 'os', value: 'Android' });
      expect(addedQueries[1]).to.deep.equal({ key: 'device', value: 'Pixel6' });
    });

    it('deletes selected pills on Backspace', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');
      const firstPill = pills[0] as any;

      // Select first pill
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      let removedKey = '';
      element.addEventListener('remove-key', (e: any) => {
        removedKey = e.detail.key;
      });

      // Dispatch Backspace on input
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLElement;
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Backspace', bubbles: true }));

      expect(removedKey).to.equal('benchmark');
    });

    it('selects all pills on Ctrl+A', async () => {
      const pills = element.shadowRoot!.querySelectorAll('explore-multi-v2-select-sk');

      // Dispatch Ctrl+A on input
      const input = element.shadowRoot!.querySelector('.query-input') as HTMLElement;
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true }));
      await element.updateComplete;

      expect((pills[0] as any).isHighlighted).to.be.true;
      expect((pills[1] as any).isHighlighted).to.be.true;
    });
  });
});
