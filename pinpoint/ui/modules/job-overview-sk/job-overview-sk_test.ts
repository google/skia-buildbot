import { expect } from 'chai';
import sinon from 'sinon';
import { JobOverviewSk } from './job-overview-sk';
import './index';
import { JobSchema } from '../../services/api';
import { MdDialog } from '@material/web/dialog/dialog';

const mockJob: JobSchema = {
  JobID: '12345',
  JobName: 'Test Job',
  Benchmark: 'speedometer',
  BotName: 'linux-perf',
  CreatedDate: '2023-01-01T00:00:00Z',
  SubmittedBy: 'test@google.com',
  JobStatus: 'Completed',
  JobType: 'Pairwise',
  AdditionalRequestParameters: {
    start_commit_githash: 'abc1234',
    end_commit_githash: 'def5678',
    bug_id: '123456',
    story_tags: 'some_tag',
    duration: '10',
  },
  MetricSummary: {},
  ErrorMessage: '',
};

describe('JobOverviewSk', () => {
  let container: HTMLElement;
  let element: JobOverviewSk;

  const setupElement = async (job: JobSchema | null) => {
    container = document.createElement('div');
    document.body.appendChild(container);
    element = document.createElement('job-overview-sk') as JobOverviewSk;
    element.job = job;
    container.appendChild(element);
    await element.updateComplete;
  };

  afterEach(() => {
    sinon.restore();
    if (container) {
      document.body.removeChild(container);
    }
  });

  it('does not render if job is null', async () => {
    await setupElement(null);
    const dialog = element.shadowRoot!.querySelector('md-dialog');

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(dialog).to.be.null;
  });

  it('renders job parameters correctly in a table', async () => {
    await setupElement(mockJob);
    const rows = element.shadowRoot!.querySelectorAll('.params-table tbody tr');
    // Benchmark, BotName, start_commit, end_commit, bug_id, story_tags
    expect(rows).to.have.lengthOf(6);

    const rowData: { [key: string]: string } = {};
    rows.forEach((row) => {
      const key = row.querySelector('th')!.textContent!.trim();
      const value = row.querySelector('td')!.textContent!.trim();
      rowData[key] = value;
    });

    expect(rowData['Benchmark']).to.equal('speedometer');
    expect(rowData['Bot Configuration']).to.equal('linux-perf');
    expect(rowData['Start Commit']).to.equal('abc1234');
    expect(rowData['End Commit']).to.equal('def5678');
    expect(rowData['Bug ID']).to.equal('123456');
    expect(rowData['Story Tags']).to.equal('some_tag');
  });

  it('filters out duration from parameters', async () => {
    await setupElement(mockJob);
    const headers = Array.from(element.shadowRoot!.querySelectorAll('.params-table th')).map((th) =>
      th.textContent!.trim()
    );
    expect(headers).to.not.include('Duration');
  });

  it('calls show on the dialog when public show() is called', async () => {
    await setupElement(mockJob);
    const dialog = element.shadowRoot!.querySelector('md-dialog') as MdDialog;
    const showSpy = sinon.spy(dialog, 'show');

    element.show();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(showSpy.calledOnce).to.be.true;
  });

  it('calls close on the dialog when the close button is clicked', async () => {
    await setupElement(mockJob);
    const dialog = element.shadowRoot!.querySelector('md-dialog') as MdDialog;
    const closeSpy = sinon.spy(dialog, 'close');

    const closeButton = element.shadowRoot!.querySelector('md-text-button') as HTMLElement;
    closeButton.click();

    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    expect(closeSpy.calledOnce).to.be.true;
  });
});
