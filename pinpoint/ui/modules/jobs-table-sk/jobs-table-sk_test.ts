import { expect } from 'chai';
import sinon from 'sinon';
import { JobsTableSk } from './jobs-table-sk';
import './index';
import { Job } from '../../services/api';

// Mock data for jobs.
const mockJobs: Job[] = [
  {
    job_id: '123',
    job_name: 'Test Job A',
    benchmark: 'benchmark.a',
    bot_name: 'bot1',
    user: 'user@google.com',
    created_date: '2023-10-27T10:00:00Z',
    job_type: 'performance',
    job_status: 'Completed',
  },
  {
    job_id: '456',
    job_name: 'Test Job B',
    benchmark: 'benchmark.b',
    bot_name: 'bot2',
    user: 'user@google.com',
    created_date: '2023-10-26T12:00:00Z',
    job_type: 'bisection',
    job_status: 'Running',
  },
];

describe('JobsTableSk', () => {
  let container: HTMLElement;
  let element: JobsTableSk;

  // Helper to create and append a new element for each test.
  const setupElement = async (
    jobs: Job[],
    sortBy: string = 'created_date',
    sortDir: 'asc' | 'desc' = 'desc'
  ) => {
    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('jobs-table-sk') as JobsTableSk;
    element.jobs = jobs;
    element.sortBy = sortBy;
    element.sortDir = sortDir;
    container.appendChild(element);
    await element.updateComplete; // Wait for initial render.
  };

  afterEach(() => {
    sinon.restore();
    if (container) {
      document.body.removeChild(container);
    }
  });

  it('renders the correct number of rows for the provided jobs', async () => {
    await setupElement(mockJobs);
    const rows = element.shadowRoot!.querySelectorAll('tbody tr');
    expect(rows).to.have.lengthOf(mockJobs.length);
  });

  it('renders job data correctly in table cells', async () => {
    await setupElement([mockJobs[0]]);
    const firstRowCells = element.shadowRoot!.querySelectorAll('tbody tr:first-child td');
    const job = mockJobs[0];

    const formattedDate = new Intl.DateTimeFormat(navigator.language, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date(job.created_date));

    expect(firstRowCells[0].textContent).to.equal(job.job_name);
    expect(firstRowCells[1].textContent).to.equal(job.benchmark);
    expect(firstRowCells[2].textContent).to.equal(job.bot_name);
    expect(firstRowCells[3].textContent).to.equal(job.user);
    expect(firstRowCells[4].textContent).to.equal(formattedDate);
    expect(firstRowCells[5].textContent).to.equal(job.job_type);
    expect(firstRowCells[6].textContent).to.equal(job.job_status);
  });

  it('dispatches sort-changed event on header click', async () => {
    await setupElement(mockJobs);
    const spy = sinon.spy();
    element.addEventListener('sort-changed', spy);

    // Click on the 'Benchmark' header (the second column).
    const benchmarkHeader = element.shadowRoot!.querySelectorAll('th')[1];
    benchmarkHeader.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(spy.calledOnce).to.be.true;
    const eventDetail = spy.firstCall.args[0].detail;
    expect(eventDetail).to.deep.equal({ sortBy: 'benchmark', sortDir: 'desc' });
  });

  it('toggles sort direction on subsequent clicks of the same header', async () => {
    await setupElement(mockJobs, 'job_name', 'desc');
    const spy = sinon.spy();
    element.addEventListener('sort-changed', spy);

    // The element is already sorted by job_name desc.
    // Click on the 'Job Name' header to toggle to 'asc'.
    const jobNameHeader = element.shadowRoot!.querySelectorAll('th')[0];
    jobNameHeader.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(spy.calledOnce).to.be.true;
    expect(spy.firstCall.args[0].detail).to.deep.equal({ sortBy: 'job_name', sortDir: 'asc' });
  });
});
