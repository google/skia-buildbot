import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { PinpointLandingPageSk } from './pinpoint-landing-page-sk';
import './index';
import { Job } from '../../services/api';
import { JobsTableSk } from '../jobs-table-sk/jobs-table-sk';
import '../jobs-table-sk';

describe('PinpointLandingPageSk', () => {
  let container: HTMLElement;
  let element: PinpointLandingPageSk;

  // Helper to create and append a new element for each test.
  const setupElement = async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-landing-page-sk') as PinpointLandingPageSk;
    container.appendChild(element);
    await fetchMock.flush(true);
    await element.updateComplete;
  };

  beforeEach(() => {
    // Mock endpoints required by the scaffold and the landing page.
    fetchMock.get('/benchmarks', ['benchmark_a', 'benchmark_b']);
    fetchMock.get('/bots?benchmark=', ['bot_1', 'bot_2']);
  });

  afterEach(() => {
    fetchMock.restore();
    if (container) {
      document.body.removeChild(container);
    }
  });

  it('shows loading indicator initially', async () => {
    // Mock a fetch that never resolves to keep the component in a loading state.
    fetchMock.get('begin:/json/jobs/list', new Promise(() => {}));

    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-landing-page-sk') as PinpointLandingPageSk;
    container.appendChild(element);
    // Allow one render cycle for the loading state to appear.
    await element.updateComplete;
    const loadingIndicator = element.shadowRoot!.querySelector('p');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(loadingIndicator).to.not.be.null;
    expect(loadingIndicator!.textContent).to.equal('Loading jobs...');
  });

  it('displays jobs when fetch is successful', async () => {
    const mockJobsPage: Job[] = [
      {
        job_id: '1',
        job_name: 'Job 1',
        benchmark: 'b1',
        bot_name: 'bot1',
        user: 'u1',
        created_date: '2023-01-01T00:00:00Z',
        job_type: 'perf',
        job_status: 'Completed',
      },
    ];
    fetchMock.get('begin:/json/jobs/list', mockJobsPage);
    await setupElement();

    const jobsTable = element.shadowRoot!.querySelector('jobs-table-sk') as JobsTableSk;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(jobsTable).to.not.be.null;
    expect(jobsTable.jobs).to.deep.equal(mockJobsPage);
  });

  it('displays an error message when fetch fails', async () => {
    const errorMessage = 'Failed to fetch';
    fetchMock.get('begin:/json/jobs/list', { throws: new Error(errorMessage) });
    await setupElement();

    const errorP = element.shadowRoot!.querySelector('p');
    expect(errorP!.textContent).to.contain(`Error: ${errorMessage}`);
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(element.shadowRoot!.querySelector('jobs-table-sk')).to.be.null;
  });

  it('handles pagination correctly', async () => {
    const fullPageOfJobs = Array.from({ length: 20 }, (_, i) => ({
      job_id: `${i}`,
    })) as Job[];
    fetchMock.get('/json/jobs/list?limit=20', fullPageOfJobs);
    await setupElement();

    let nextButton = element.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    let prevButton = element.shadowRoot!.querySelector('md-outlined-button') as HTMLElement;

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(nextButton, 'Next button should exist').to.not.be.null;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(prevButton, 'Previous button should exist').to.not.be.null;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(nextButton.hasAttribute('disabled')).to.be.false;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(prevButton.hasAttribute('disabled')).to.be.true;

    const secondPageUrl = '/json/jobs/list?limit=20&offset=20';
    fetchMock.get(secondPageUrl, [{ job_id: '21' } as Job]);
    nextButton.click();
    await fetchMock.flush(true);
    await element.updateComplete;

    // Verify the second page was requested.
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called(secondPageUrl)).to.be.true;

    nextButton = element.shadowRoot!.querySelector('md-filled-button') as HTMLElement;
    prevButton = element.shadowRoot!.querySelector('md-outlined-button') as HTMLElement;

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(nextButton.hasAttribute('disabled')).to.be.true;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(prevButton.hasAttribute('disabled')).to.be.false;
  });

  it('refetches jobs on search change', async () => {
    // Mock initial fetch.
    fetchMock.get('/json/jobs/list?limit=20', []);
    await setupElement();

    const searchUrl = '/json/jobs/list?search_term=new+search&limit=20';
    fetchMock.get(searchUrl, []);

    const scaffold = element.shadowRoot!.querySelector('pinpoint-scaffold-sk')!;
    scaffold.dispatchEvent(new CustomEvent('search-changed', { detail: { value: 'new search' } }));
    await fetchMock.flush(true);
    await element.updateComplete;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(fetchMock.called(searchUrl)).to.be.true;
  });

  it('sorts jobs client-side on sort-changed event', async () => {
    const jobsToSort: Job[] = [
      { job_id: '1', job_name: 'B Job', created_date: '2023-01-01T00:00:00Z' } as Job,
      { job_id: '2', job_name: 'A Job', created_date: '2023-01-02T00:00:00Z' } as Job,
    ];
    fetchMock.get('begin:/json/jobs/list', jobsToSort);
    await setupElement();

    let jobsTable = element.shadowRoot!.querySelector('jobs-table-sk')! as JobsTableSk;
    // Default sort is created_date desc.
    expect(jobsTable.jobs[0].job_name).to.equal('A Job');

    // Dispatch sort-changed event to sort by job_name asc.
    jobsTable.dispatchEvent(
      new CustomEvent('sort-changed', {
        detail: { sortBy: 'job_name', sortDir: 'asc' },
      })
    );
    await element.updateComplete;

    jobsTable = element.shadowRoot!.querySelector('jobs-table-sk')! as JobsTableSk;
    expect(jobsTable.jobs[0].job_name).to.equal('A Job');
    expect(jobsTable.sortBy).to.equal('job_name');
    expect(jobsTable.sortDir).to.equal('asc');
  });
});
