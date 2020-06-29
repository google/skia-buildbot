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
});
