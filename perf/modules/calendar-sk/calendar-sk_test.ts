import './index';
import { assert } from 'chai';
import { CalendarSk } from './calendar-sk';
import {
  setUpElementUnderTest,
  eventPromise,
} from '../../../infra-sk/modules/test_util';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('calendar-sk', () => {
  const newInstance = setUpElementUnderTest<CalendarSk>('calendar-sk');
  let calendarSk: CalendarSk;
  beforeEach(() => {
    calendarSk = newInstance();
    calendarSk.displayDate = new Date(2020, 4, 21);
  });

  describe('event', () => {
    it('fires when button is clicked', () => window.customElements.whenDefined('calendar-sk').then(async () => {
      const event = eventPromise<CustomEvent<Date>>('change');
        calendarSk
          .querySelector<HTMLButtonElement>('button[data-date="19"]')!
          .click();

        const detail = (await event).detail;

        assert.equal(
          new Date(2020, 4, 19).toDateString(),
          detail.toDateString(),
          'Date has changed.',
        );
        assert.equal(getSelectedDate(), 19, 'Selected date has changed.');
    }));
  });
  describe('year', () => {
    it('decrements when button is clicked', () => window.customElements.whenDefined('calendar-sk').then(() => {
        calendarSk.querySelector<HTMLButtonElement>('#previous-year')!.click();

        const year = calendarSk.querySelector<HTMLHeadingElement>(
          '#calendar-year',
        )!.innerText;
        assert.equal(year, '2019', 'Year has changed.');
        assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
    it('increments when button is clicked', () => window.customElements.whenDefined('calendar-sk').then(() => {
        calendarSk.querySelector<HTMLButtonElement>('#next-year')!.click();

        const year = calendarSk.querySelector<HTMLHeadingElement>(
          '#calendar-year',
        )!.innerText;
        assert.equal(year, '2021', 'Year has changed.');
        assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
  });
  describe('month', () => {
    it('decrements when button is clicked', () => window.customElements.whenDefined('calendar-sk').then(() => {
        calendarSk.querySelector<HTMLButtonElement>('#previous-month')!.click();

        const year = calendarSk.querySelector<HTMLHeadingElement>(
          '#calendar-month',
        )!.innerText;
        assert.equal(year, 'April', 'Month has changed.');
        assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
    it('increments when button is clicked', () => window.customElements.whenDefined('calendar-sk').then(() => {
        calendarSk.querySelector<HTMLButtonElement>('#next-month')!.click();

        const year = calendarSk.querySelector<HTMLHeadingElement>(
          '#calendar-month',
        )!.innerText;
        assert.equal(year, 'June', 'Month has changed.');
        assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
  });
  describe('keyboard', () => {
    it('moves to next day when ArrowRight is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'ArrowRight' }),
      );

      assert.equal(
        new Date(2020, 4, 22).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 22, 'Selected date has changed.');
    }));
    it('moves to previous day when ArrowLeft is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'ArrowLeft' }),
      );

      assert.equal(
        new Date(2020, 4, 20).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 20, 'Selected date has changed.');
    }));
    it('moves to previous week when ArrowUp is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'ArrowUp' }),
      );

      assert.equal(
        new Date(2020, 4, 14).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 14, 'Selected date has changed.');
    }));
    it('moves to next week when ArrowDown is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'ArrowDown' }),
      );

      assert.equal(
        new Date(2020, 4, 28).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 28, 'Selected date has changed.');
    }));

    it('moves to previous month when PageUp is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'PageUp' }),
      );

      assert.equal(
        new Date(2020, 3, 21).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
    it('moves to next month when PageDown is pressed', () => window.customElements.whenDefined('calendar-sk').then(() => {
      calendarSk.keyboardHandler(
        new KeyboardEvent('keydown', { code: 'PageDown' }),
      );

      assert.equal(
        new Date(2020, 5, 21).toDateString(),
        calendarSk.displayDate.toDateString(),
        'Date has changed.',
      );
      assert.equal(getSelectedDate(), 21, 'Selected date has not changed.');
    }));
  });

  const getSelectedDate = () => +calendarSk.querySelector<HTMLButtonElement>(
    '[aria-selected="true"]',
  )!.innerText;
});
