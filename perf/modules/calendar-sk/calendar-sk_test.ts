import './index';
import { assert } from 'chai';
import { CalendarSk, CalendarSkChangeEventDetail } from './calendar-sk';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('calendar-sk', () => {
  describe('event', () => {
    it('fires when button is clicked', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        let value: Date = new Date();
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.addEventListener('change', (e) => {
          value = (e as CustomEvent<CalendarSkChangeEventDetail>).detail.date;
        });
        ele.querySelector<HTMLButtonElement>('button[data-date="19"]')!.click();
        assert.equal(
          new Date(2020, 4, 19).toDateString(),
          value.toDateString(),
          'Date has changed.'
        );
      }));
  });
  describe('year', () => {
    it('decrements when button is clicked', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.querySelector<HTMLButtonElement>('#previous-year')!.click();

        const year = ele.querySelector<HTMLHeadingElement>('#calendar-year')!
          .innerText;
        assert.equal(year, '2019', 'Year has changed.');
      }));
    it('increments when button is clicked', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.querySelector<HTMLButtonElement>('#next-year')!.click();

        const year = ele.querySelector<HTMLHeadingElement>('#calendar-year')!
          .innerText;
        assert.equal(year, '2021', 'Year has changed.');
      }));
  });
  describe('month', () => {
    it('decrements when button is clicked', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.querySelector<HTMLButtonElement>('#previous-month')!.click();

        const year = ele.querySelector<HTMLHeadingElement>('#calendar-month')!
          .innerText;
        assert.equal(year, 'April', 'Month has changed.');
      }));
    it('increments when button is clicked', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.querySelector<HTMLButtonElement>('#next-month')!.click();

        const year = ele.querySelector<HTMLHeadingElement>('#calendar-month')!
          .innerText;
        assert.equal(year, 'June', 'Month has changed.');
      }));
  });
  describe('keyboard', () => {
    it('moves to next day when ArrowRight is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(
          new KeyboardEvent('keydown', { code: 'ArrowRight' })
        );

        assert.equal(
          new Date(2020, 4, 22).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));
    it('moves to previous day when ArrowLeft is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(
          new KeyboardEvent('keydown', { code: 'ArrowLeft' })
        );

        assert.equal(
          new Date(2020, 4, 20).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));
    it('moves to previous week when ArrowUp is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(new KeyboardEvent('keydown', { code: 'ArrowUp' }));

        assert.equal(
          new Date(2020, 4, 14).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));
    it('moves to next week when ArrowDown is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(
          new KeyboardEvent('keydown', { code: 'ArrowDown' })
        );

        assert.equal(
          new Date(2020, 4, 28).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));

    it('moves to previous month when PageUp is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(new KeyboardEvent('keydown', { code: 'PageUp' }));

        assert.equal(
          new Date(2020, 3, 21).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));
    it('moves to next month when PageDown is pressed', () =>
      window.customElements.whenDefined('calendar-sk').then(() => {
        container.innerHTML = '<calendar-sk value=untriaged></calendar-sk>';
        const ele = container.firstElementChild! as CalendarSk;
        ele.displayDate = new Date(2020, 4, 21);

        ele.keyboardHandler(new KeyboardEvent('keydown', { code: 'PageDown' }));

        assert.equal(
          new Date(2020, 5, 21).toDateString(),
          ele.displayDate.toDateString(),
          'Date has changed.'
        );
      }));
  });
});
