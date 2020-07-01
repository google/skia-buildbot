import './index';
import { assert } from 'chai';
import { CalendarInputSk } from './calendar-input-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('calendar-input-sk', () => {
  const newInstance = setUpElementUnderTest<CalendarInputSk>(
    'calendar-input-sk'
  );
  let calendarInputSk: CalendarInputSk;
  beforeEach(() => {
    calendarInputSk = newInstance();
    calendarInputSk.displayDate = new Date(2020, 4, 21);
  });

  describe('input control', () => {
    it('displays the date correctly', () =>
      window.customElements.whenDefined('calendar-input-sk').then(async () => {
        assert.equal(
          calendarInputSk.querySelector<HTMLInputElement>('input')!.value,
          '2020-5-21'
        );
      }));
  });
});
