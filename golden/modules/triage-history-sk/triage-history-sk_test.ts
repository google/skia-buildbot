import './index';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { TriageHistorySk } from './triage-history-sk';
import { expect } from 'chai';

describe('triage-history-sk', () => {
  const newInstance = setUpElementUnderTest<TriageHistorySk>('triage-history-sk');

  const originalNow = Date.now;
  const originalDateToString = Date.prototype.toString;

  let triageHistorySk: TriageHistorySk;

  beforeEach(() => {
    Date.now = () => new Date('2020-03-05T05:06:07-04:00').valueOf();
    // This makes the test deterministic w.r.t. the computer's timezone.
    Date.prototype.toString = Date.prototype.toUTCString;
    triageHistorySk = newInstance();
  });

  afterEach(() => {
    Date.now = originalNow;
    Date.prototype.toString = originalDateToString;
  });

  it('renders nothing on an empty history', () => {
    triageHistorySk.history = [];
    expect(triageHistorySk.innerText).to.be.empty;
  });

  it('renders only the most recent history entry', () => {
    triageHistorySk.history = [{
      user: 'helpfuluser@example.com',
      ts: '2020-03-04T05:06:07.000000000-04:00',
    }, {
      user: 'thisshouldnotshowup@example.com',
      ts: '2020-01-01T05:06:07.000000000-04:00',
    }];
    expect(triageHistorySk.innerText).to.equal('1d ago by helpfuluser@');
    const msg = triageHistorySk.querySelector<HTMLDivElement>('.message')!;
    expect(msg.getAttribute('title')).to.equal(
      'Last triaged on Wed, 04 Mar 2020 09:06:07 GMT by helpfuluser@example.com',
    );
  });

  it('renders the full user name if it is not an email', () => {
    triageHistorySk.history = [{
      user: 'expectation_cleaner',
      ts: '2020-03-04T05:06:07.000000000-04:00',
    }];
    expect(triageHistorySk.innerText).to.equal('1d ago by expectation_cleaner');
  });
});
