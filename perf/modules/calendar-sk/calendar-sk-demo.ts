import './index.ts';
import { CalendarSk } from './calendar-sk';

document.querySelectorAll<CalendarSk>('calendar-sk').forEach((ele) => {
  ele.date = new Date(2020, 4, 20);
  ele.displayDate = new Date(2020, 4, 21);
});
