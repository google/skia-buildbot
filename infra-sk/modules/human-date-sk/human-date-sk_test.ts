import { expect } from 'chai';
import { HumanDateSk } from './human-date-sk';
import './index';

import { setUpElementUnderTest } from '../test_util';
import { $$ } from '../dom';

describe('human-date-sk', () => {
  const newInstance = setUpElementUnderTest<HumanDateSk>('human-date-sk');

  let element: HumanDateSk;
  let nowImpl = () => new Date('September 22, 2020 10:21:52').getTime();
  const localeImpl = global;
  beforeEach(() => {
    [nowImpl, Date.now] = [Date.now, nowImpl];
  });
  afterEach(() => {
    [nowImpl, Date.now] = [Date.now, nowImpl];
  });
  describe('human-date-sk', () => {
    it('displays number of seconds', () => {
      element = newInstance((el: HumanDateSk) => {
        el.date = 1600784512;
        el.seconds = true;
      });
      // Make sure everything works but the hour, easier than mocking Date.toLocale*.
      expect((<HTMLElement>$$('span', element)).innerText)
        .to.contain('9/22/2020')
        .and.to.contain(':21:52 ');
    });

    it('displays number of millis', () => {
      element = newInstance((el: HumanDateSk) => {
        el.date = 1600784512000;
      });
      // Make sure everything works but the hour, easier than mocking Date.toLocale*.
      expect((<HTMLElement>$$('span', element)).innerText)
        .to.contain('9/22/2020')
        .and.to.contain(':21:52 ');
    });

    it('displays string', () => {
      element = newInstance((el: HumanDateSk) => {
        el.date = 'October 31, 2020 23:59:59';
      });
      expect($$('span', element)).to.have.property('innerText', '10/31/2020, 11:59:59 PM');
    });

    it('displays diff', () => {
      element = newInstance((el: HumanDateSk) => {
        el.date = 'September 22, 2020 04:21:52';
        el.diff = true;
      });
      expect($$('span', element)).to.have.property('innerText', '6h ago');
    });
  });
});
