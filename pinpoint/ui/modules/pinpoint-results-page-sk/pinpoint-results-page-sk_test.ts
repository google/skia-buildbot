import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ResultsPageSk } from './pinpoint-results-page-sk';
import './index';
import { JobSchema } from '../../services/api';
import { JobOverviewSk } from '../job-overview-sk/job-overview-sk';

const mockJob: JobSchema = {
  JobID: '12345',
  JobName: 'Test Pairwise Job',
  JobStatus: 'Completed',
  JobType: 'Pairwise',
  SubmittedBy: 'test@google.com',
  Benchmark: 'speedometer',
  BotName: 'linux-perf',
  CreatedDate: '2023-01-01T12:00:00Z',
  ErrorMessage: '',
  AdditionalRequestParameters: {
    duration: '120',
    commit_runs: {
      left: {
        Build: null,
        Runs: [],
      },
      right: {
        Build: null,
        Runs: [],
      },
    },
  },
  MetricSummary: {
    some_metric: {
      p_value: 0.01,
      confidence_interval_lower: 1.0,
      confidence_interval_higher: 2.0,
      control_median: 100,
      treatment_median: 105,
      significant: true,
    },
  },
};

describe('ResultsPageSk', () => {
  let container: HTMLElement;
  let element: ResultsPageSk;
  const originalPath = window.location.pathname;

  beforeEach(() => {
    // Mock window.location to provide a job ID in the path.
    history.pushState(null, '', '/results/jobid/12345');
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
    container?.remove();
    // Restore original window.location path.
    history.pushState(null, '', originalPath);
  });

  const setupElement = async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-results-page-sk') as ResultsPageSk;
    container.appendChild(element);
    // Wait for fetches to complete and the element to render.
    await fetchMock.flush(true);
    await element.updateComplete;
  };

  it('shows loading indicator initially', async () => {
    fetchMock.get('/json/job/12345', new Promise(() => {}));
    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('pinpoint-results-page-sk') as ResultsPageSk;
    container.appendChild(element);
    await element.updateComplete;
    const loadingIndicator = element.shadowRoot!.querySelector('p');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(loadingIndicator).to.not.be.null;
    expect(loadingIndicator!.textContent).to.equal('Loading...');
  });

  it('displays an error if job ID is missing from URL', async () => {
    history.pushState(null, '', '/results/jobid/'); // No job ID
    // No fetch mock needed as it should fail before fetching.
    await setupElement();
    const errorIndicator = element.shadowRoot!.querySelector('p');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(errorIndicator).to.not.be.null;
    expect(errorIndicator!.textContent).to.contain('No Job ID found in URL.');
  });

  it('displays an error if fetching the job fails', async () => {
    fetchMock.get('/json/job/12345', { throws: new Error('API Error') });
    await setupElement();
    const errorIndicator = element.shadowRoot!.querySelector('p');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(errorIndicator).to.not.be.null;
    expect(errorIndicator!.textContent).to.contain('Failed to fetch job: API Error');
  });

  it('renders job details correctly on successful fetch', async () => {
    fetchMock.get('/json/job/12345', mockJob);
    await setupElement();

    const title = element.shadowRoot!.querySelector('.title');
    expect(title!.textContent).to.contain(mockJob.JobName);

    const formattedDate = new Intl.DateTimeFormat(navigator.language, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date(mockJob.CreatedDate));

    const subtitleSpans = element.shadowRoot!.querySelectorAll('.subtitle span');
    expect(subtitleSpans[0].textContent).to.contain(mockJob.SubmittedBy);
    expect(subtitleSpans[1].textContent).to.contain(formattedDate);
    expect(subtitleSpans[2].textContent).to.contain('120 minutes');
  });

  it('renders wilcoxon results for completed pairwise jobs', async () => {
    fetchMock.get('/json/job/12345', mockJob);
    await setupElement();
    const wilcoxonElement = element.shadowRoot!.querySelector('wilcoxon-result-sk');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(wilcoxonElement).to.not.be.null;
  });

  it('renders pending status for running jobs', async () => {
    const mockRunningJob = { ...mockJob, JobStatus: 'Running' };
    fetchMock.get('/json/job/12345', mockRunningJob);
    await setupElement();
    const statusBox = element.shadowRoot!.querySelector('.status-box');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(statusBox).to.not.be.null;
    expect(statusBox!.textContent).to.contain('Results Pending');
    expect(statusBox!.textContent).to.contain('Job is currently running');
  });

  it('renders failed status for failed jobs', async () => {
    const mockFailedJob = { ...mockJob, JobStatus: 'Failed', ErrorMessage: 'It broke' };
    fetchMock.get('/json/job/12345', mockFailedJob);
    await setupElement();
    const errorBox = element.shadowRoot!.querySelector('.error-box');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(errorBox).to.not.be.null;
    expect(errorBox!.textContent).to.contain('Job Failed');
    expect(errorBox!.textContent).to.contain('Error: It broke');
  });

  it('renders canceled status for canceled jobs', async () => {
    const mockCanceledJob = { ...mockJob, JobStatus: 'Canceled' };
    fetchMock.get('/json/job/12345', mockCanceledJob);
    await setupElement();
    const statusBox = element.shadowRoot!.querySelector('.status-box');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(statusBox).to.not.be.null;
    expect(statusBox!.textContent).to.contain('Job Canceled');
  });

  it('does not render wilcoxon results for non-pairwise jobs', async () => {
    const mockNonPairwiseJob = { ...mockJob, JobType: 'Bisection' };
    fetchMock.get('/json/job/12345', mockNonPairwiseJob);
    await setupElement();
    const wilcoxonElement = element.shadowRoot!.querySelector('wilcoxon-result-sk');
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(wilcoxonElement).to.be.null;
  });

  it('opens arguments dialog on button click', async () => {
    fetchMock.get('/json/job/12345', mockJob);
    await setupElement();

    const jobOverview = element.shadowRoot!.querySelector('job-overview-sk') as JobOverviewSk;
    const showSpy = sinon.spy(jobOverview, 'show');

    const viewArgsButton = element.shadowRoot!.querySelector('md-outlined-button') as HTMLElement;
    viewArgsButton.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(showSpy.calledOnce).to.be.true;
  });
});
