import './index';
import { $$ } from 'common-sk/modules/dom';
import {
  setUpElementUnderTest,
} from '../test_util';

describe('triage-history-sk', () => {
  const newInstance = setUpElementUnderTest('triage-history-sk');

  const originalNow = Date.now;
  let triageHistorySk;
  beforeEach(() => {
    Date.now = () => new Date('2020-03-05T05:06:07-04:00');
    triageHistorySk = newInstance();
  });

  afterEach(() => {
    Date.now = originalNow;
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

  describe('html', () => {
    beforeEach(() => {
      triageHistorySk.history = [{
        user: 'helpfuluser@example.com',
        ts: new Date('2020-03-04T05:06:07.000000000-04:00'),
      }];
    });

    it('has title text', () => {
      const msg = $$('.message', triageHistorySk);
      expect(msg.innerText).to.equal('1d ago by helpfuluser@');
    });

    it('has title text which is more verbose', () => {
      const msg = $$('.message', triageHistorySk);
      expect(msg.getAttribute('title')).to.equal(
        'Last triaged on Wed Mar 04 2020 04:06:07 GMT-0500 (Eastern Standard Time) by helpfuluser@example.com',
      );
    });
  });
});
