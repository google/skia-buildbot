import './index';
import { expect } from 'chai';
import { CommitDetailSk } from './commit-detail-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Commit, CommitNumber } from '../json';
import sinon from 'sinon';

describe('commit-detail-sk', () => {
  const newInstance = setUpElementUnderTest<CommitDetailSk>('commit-detail-sk');

  let element: CommitDetailSk;
  const commit: Commit = {
    hash: 'e57303f2',
    offset: CommitNumber(12345),
    author: 'user@example.com',
    message: 'Fix bug',
    ts: 1678886400, // 2023-03-15T13:20:00Z
    url: 'http://example.com/commit',
    body: '',
  };

  let windowOpenStub: sinon.SinonStub;

  beforeEach(() => {
    windowOpenStub = sinon.stub(window, 'open');
    element = newInstance((el: CommitDetailSk) => {
      el.cid = commit;
    });
  });

  afterEach(() => {
    windowOpenStub.restore();
  });

  it('renders commit details correctly', () => {
    expect(element.innerText).to.contain('e57303f2');
    expect(element.innerText).to.contain('user@example.com');
    expect(element.innerText).to.contain('Fix bug');
  });

  it('opens generic explore link when trace_id is not set', async () => {
    const exploreBtn = element.querySelectorAll<HTMLElement>('md-outlined-button')[0];
    expect(exploreBtn.textContent).to.contain('Explore');

    exploreBtn.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(windowOpenStub.called).to.be.true;
    expect(windowOpenStub.lastCall.args[0]).to.equal('/g/e/e57303f2');
  });

  it('opens specific explore link when trace_id is set', async () => {
    element.trace_id = 'test_trace_id';
    await new Promise((resolve) => setTimeout(resolve, 0));
    const exploreBtn = element.querySelectorAll<HTMLElement>('md-outlined-button')[0];

    exploreBtn.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(windowOpenStub.called).to.be.true;
    const url = windowOpenStub.lastCall.args[0] as string;

    expect(url).to.contain('/e/?');
    expect(url).to.contain('keys=test_trace_id');
    expect(url).to.contain('xbaroffset=12345');
    expect(url).to.contain('begin=1678540800');
    expect(url).to.contain('end=1679232000');
  });

  it('opens cluster link', async () => {
    const clusterBtn = element.querySelectorAll<HTMLElement>('md-outlined-button')[1];
    expect(clusterBtn.textContent).to.contain('Cluster');

    clusterBtn.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(windowOpenStub.called).to.be.true;
    expect(windowOpenStub.lastCall.args[0]).to.equal('/g/c/e57303f2');
  });

  it('opens triage link', async () => {
    const triageBtn = element.querySelectorAll<HTMLElement>('md-outlined-button')[2];
    expect(triageBtn.textContent).to.contain('Triage');

    triageBtn.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(windowOpenStub.called).to.be.true;
    expect(windowOpenStub.lastCall.args[0]).to.equal('/g/t/e57303f2');
  });

  it('opens commit link', async () => {
    const commitBtn = element.querySelectorAll<HTMLElement>('md-outlined-button')[3];
    expect(commitBtn.textContent).to.contain('Commit');

    commitBtn.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(windowOpenStub.called).to.be.true;
    expect(windowOpenStub.lastCall.args[0]).to.equal('http://example.com/commit');
  });
});
