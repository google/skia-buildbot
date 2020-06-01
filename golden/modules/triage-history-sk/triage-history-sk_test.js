import './index';
import { $$ } from 'common-sk/modules/dom';
import {
  setUpElementUnderTest,
} from '../test_util';

describe('triage-history-sk', () => {
  const newInstance = setUpElementUnderTest('triage-history-sk');

  const originalNow = Date.now;
  const originalDateToString = Date.prototype.toString;

  let triageHistorySk;

  beforeEach(() => {
    Date.now = () => new Date('2020-03-05T05:06:07-04:00');
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
      ts: new Date('2020-03-04T05:06:07.000000000-04:00'),
    }, {
      user: 'thisshouldnotshowup@example.com',
      ts: new Date('2020-01-01T05:06:07.000000000-04:00'),
    }];
    expect(triageHistorySk.innerText).to.equal('1d ago by helpfuluser@');
    const msg = $$('.message', triageHistorySk);
    expect(msg.getAttribute('title')).to.equal(
      'Last triaged on Wed, 04 Mar 2020 09:06:07 GMT by helpfuluser@example.com',
    );
  });

  it('renders the full user name if it is not an email', () => {
    triageHistorySk.history = [{
      user: 'expectation_cleaner',
      ts: new Date('2020-03-04T05:06:07.000000000-04:00'),
    }];
    expect(triageHistorySk.innerText).to.equal('1d ago by expectation_cleaner');
  });

  describe('"history" property setter/getter', () => {
    it('converts date strings into real dates', () => {
      triageHistorySk.history = [{
        user: 'helpfuluser@example.com',
        ts: '2020-03-04T05:06:07.000000000-04:00',
      }];

      expect(triageHistorySk.history.length).to.equal(1);
      expect(triageHistorySk.history[0].user).to.equal('helpfuluser@example.com');
      expect(triageHistorySk.history[0].ts.getTime()).to.equal(
        Date.parse('2020-03-04T05:06:07.000000000-04:00'),
      );
    });
  });
});
