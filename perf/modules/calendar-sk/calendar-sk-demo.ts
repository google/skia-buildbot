import './index.ts';
import { CalendarSk, CalendarSkChangeEventDetail } from './calendar-sk';

const evt = document.getElementById('evt')!;
const locales = [undefined, undefined, 'de-DE', 'zh-Hans-CN'];
document.querySelectorAll<CalendarSk>('calendar-sk').forEach((ele, i) => {
  ele.date = new Date(2020, 4, 20);
  ele.displayDate = new Date(2020, 4, 21);
  ele.locale = locales[i];
  ele.addEventListener('change', (e) => {
    evt.innerText = (e as CustomEvent<
      CalendarSkChangeEventDetail
    >).detail.date.toString();
  });
});
