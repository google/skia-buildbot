import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { PinpointNewJobSk } from './pinpoint-new-job-sk';
import './index';
import type { Tabs } from '@material/web/tabs/internal/tabs.js';
import type { MdOutlinedSelect } from '@material/web/select/outlined-select.js';

describe('PinpointNewJobSk', () => {
  let container: HTMLElement;
  let element: PinpointNewJobSk;

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
    if (container) {
      document.body.removeChild(container);
    }
  });

  const setupElement = async () => {
    fetchMock.get('/benchmarks', ['benchmark_a', 'benchmark_b']);
    fetchMock.get('/bots?benchmark=', ['bot_1', 'bot_2']);

    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-new-job-sk') as PinpointNewJobSk;
    container.appendChild(element);
    await fetchMock.flush(true);
    await element.updateComplete;
  };

  it('fetches initial data on connectedCallback', async () => {
    await setupElement();
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called('/benchmarks')).to.be.true;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called('/bots?benchmark=')).to.be.true;
  });

  it('shows and closes the dialog', async () => {
    await setupElement();
    const dialog = element.shadowRoot!.querySelector('md-dialog')!;
    const showSpy = sinon.spy(dialog, 'show');
    const closeSpy = sinon.spy(dialog, 'close');

    element.show();
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(showSpy.calledOnce).to.be.true;

    const cancelButton = element.shadowRoot!.querySelector('md-outlined-button') as HTMLElement;
    cancelButton.click();
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(closeSpy.calledOnce).to.be.true;
  });

  it('switches between detailed and simplified tabs', async () => {
    await setupElement();
    // Default is detailed view, which has an 'about-section'.
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(element.shadowRoot!.querySelector('.about-section')).to.not.be.null;

    const tabs = element.shadowRoot!.querySelector('md-tabs') as Tabs;
    tabs.activeTabIndex = 1; // Switch to "Simplified"
    tabs.dispatchEvent(new CustomEvent('change'));
    await element.updateComplete;

    // Simplified view does not have 'about-section'.
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(element.shadowRoot!.querySelector('.about-section')).to.be.null;
    const h2 = element.shadowRoot!.querySelector('h2');
    expect(h2!.textContent).to.contain('1. Define Commit Range');
  });

  it('updates bots and stories when a benchmark is selected', async () => {
    fetchMock.get('/bots?benchmark=benchmark_a', ['bot_a1', 'bot_a2']);
    fetchMock.get('/stories?benchmark=benchmark_a', ['story_a1', 'story_a2']);
    await setupElement();

    const benchmarkSelect = element.shadowRoot!.querySelector(
      'md-outlined-select[label="Benchmark"]'
    ) as MdOutlinedSelect;
    benchmarkSelect.value = 'benchmark_a';
    benchmarkSelect.dispatchEvent(new Event('change'));
    await fetchMock.flush(true);
    await element.updateComplete;

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called('/bots?benchmark=benchmark_a')).to.be.true;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called('/stories?benchmark=benchmark_a')).to.be.true;

    await element.updateComplete;
    const botOptions = element.shadowRoot!.querySelectorAll(
      'md-outlined-select[label="Device to test on"] md-select-option'
    );
    expect(botOptions.length).to.equal(2);
    expect(botOptions[0].textContent!.trim()).to.equal('bot_a1');
  });

  it('resets dependent fields when benchmark is deselected', async () => {
    fetchMock.get('/bots?benchmark=benchmark_a', ['bot_a1']);
    fetchMock.get('/stories?benchmark=benchmark_a', ['story_a1']);
    await setupElement();

    const benchmarkSelect = element.shadowRoot!.querySelector(
      'md-outlined-select[label="Benchmark"]'
    ) as MdOutlinedSelect;
    benchmarkSelect.value = 'benchmark_a';
    benchmarkSelect.dispatchEvent(new Event('change'));
    await fetchMock.flush(true);
    await element.updateComplete;

    // Now deselect.
    benchmarkSelect.value = '';
    benchmarkSelect.dispatchEvent(new Event('change'));
    await fetchMock.flush(true);
    await element.updateComplete;

    // Check that listBots was called again with an empty benchmark.
    // The first call is in setupElement, the second is from the deselection.
    expect(fetchMock.calls('/bots?benchmark=').length).to.equal(2);

    const botOptions = element.shadowRoot!.querySelectorAll(
      'md-outlined-select[label="Device to test on"] md-select-option'
    );
    expect(botOptions.length).to.equal(2);
    expect(botOptions[0].textContent!.trim()).to.equal('bot_1');

    const storyOptions = element.shadowRoot!.querySelectorAll(
      'md-outlined-select[label="Story"] md-select-option'
    );
    expect(storyOptions.length).to.equal(0);
  });
});
