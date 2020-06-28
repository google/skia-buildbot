import './index.ts';
import { CalendarSk, CalendarSkChangeEventDetail } from './calendar-sk';

const evt = document.getElementById('evt')!;
document.querySelectorAll<CalendarSk>('calendar-sk').forEach((ele) => {
  ele.date = new Date(2020, 4, 20);
  ele.displayDate = new Date(2020, 4, 21);
  ele.addEventListener('change', (e) => {
    evt.innerText = (e as CustomEvent<
      CalendarSkChangeEventDetail
    >).detail.date.toString();
  });
});
