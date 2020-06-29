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
});
