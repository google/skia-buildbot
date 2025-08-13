import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { PinpointScaffoldSk } from './pinpoint-scaffold-sk';
import './index';
import { PinpointNewJobSk } from '../pinpoint-new-job-sk/pinpoint-new-job-sk';
import { ComboBox } from '@vaadin/combo-box';

describe('PinpointScaffoldSk', () => {
  const benchmarks = ['benchmark1', 'benchmark2'];
  const bots = ['bot1', 'bot2'];

  let container: HTMLElement;
  let element: PinpointScaffoldSk;

  beforeEach(async () => {
    fetchMock.get('/benchmarks', benchmarks);
    fetchMock.get('/bots?benchmark=', bots);

    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-scaffold-sk') as PinpointScaffoldSk;
    container.appendChild(element);

    // Wait for element to connect and fetch initial data.
    await fetchMock.flush(true);
    await element.updateComplete;
  });

  afterEach(() => {
    fetchMock.reset();
    sinon.restore();
    document.body.removeChild(container);
  });

  it('loads initial data on connectedCallback', () => {
    // @ts-expect-error - access private property for testing
    expect(element._benchmarks).to.deep.equal(benchmarks);
    // @ts-expect-error - access private property for testing
    expect(element._bots).to.deep.equal(bots);
  });

  it('dispatches search-changed event on search input', () => {
    const spy = sinon.spy();
    element.addEventListener('search-changed', spy);

    const searchField = element.shadowRoot!.querySelector(
      'md-outlined-text-field[label="Search by job name"]'
    ) as HTMLElement;
    const inputElement = searchField.shadowRoot!.querySelector('input')!;
    inputElement.value = 'test search';
    inputElement.dispatchEvent(new Event('input', { bubbles: true, composed: true }));

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(spy.calledOnce).to.be.true;
    expect(spy.firstCall.args[0].detail).to.deep.equal({ value: 'test search' });
  });

  it('dispatches filters-changed event when filters are applied', async () => {
    const spy = sinon.spy();
    element.addEventListener('filters-changed', spy);

    // Open the filter menu
    const filterButton = element.shadowRoot!.querySelector('#filter-anchor') as HTMLElement;
    filterButton.click();
    await element.updateComplete;

    // Set filter values
    const benchmarkComboBox = element.shadowRoot!.querySelector(
      'vaadin-combo-box[label="Benchmark"]'
    ) as ComboBox;
    benchmarkComboBox.value = 'benchmark1';
    benchmarkComboBox.dispatchEvent(
      new CustomEvent('value-changed', { detail: { value: 'benchmark1' } })
    );

    const botComboBox = element.shadowRoot!.querySelector(
      'vaadin-combo-box[label="Device"]'
    ) as ComboBox;
    botComboBox.value = 'bot1';
    botComboBox.dispatchEvent(new CustomEvent('value-changed', { detail: { value: 'bot1' } }));

    const userFilter = element.shadowRoot!.querySelector(
      '.filter-menu-items md-outlined-text-field[label="User"]'
    ) as HTMLElement;
    const inputElement = userFilter.shadowRoot!.querySelector('input')!;
    inputElement.value = 'test-user';
    inputElement.dispatchEvent(new Event('input', { bubbles: true, composed: true }));

    const startDateFilter = element.shadowRoot!.querySelector(
      '.filter-menu-items md-outlined-text-field[label="Start Date"]'
    ) as HTMLElement;
    const startDateInputElement = startDateFilter.shadowRoot!.querySelector('input')!;
    startDateInputElement.value = '2023-10-01';
    startDateInputElement.dispatchEvent(new Event('input', { bubbles: true, composed: true }));

    const endDateFilter = element.shadowRoot!.querySelector(
      '.filter-menu-items md-outlined-text-field[label="End Date"]'
    ) as HTMLElement;
    const endDateInputElement = endDateFilter.shadowRoot!.querySelector('input')!;
    endDateInputElement.value = '2023-10-31';
    endDateInputElement.dispatchEvent(new Event('input', { bubbles: true, composed: true }));

    await element.updateComplete;

    // Apply filters
    const applyButton = element.shadowRoot!.querySelector(
      '.filter-actions md-filled-button'
    ) as HTMLElement;
    applyButton.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(spy.calledOnce).to.be.true;
    expect(spy.firstCall.args[0].detail).to.deep.equal({
      benchmark: 'benchmark1',
      botName: 'bot1',
      user: 'test-user',
      startDate: '2023-10-01',
      endDate: '2023-10-31',
    });
  });

  it('opens the new job modal when "Create a new job" is clicked', () => {
    const newJobModal = element.shadowRoot!.querySelector(
      'pinpoint-new-job-sk'
    ) as PinpointNewJobSk;
    const showSpy = sinon.spy(newJobModal, 'show');

    const createButton = element.shadowRoot!.querySelector(
      '.header-actions > md-filled-button'
    ) as HTMLElement;
    createButton.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(showSpy.calledOnce).to.be.true;
  });
});
