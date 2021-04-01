import './index';
import { CalendarSk } from './calendar-sk';

const evt = document.getElementById('evt')!;
const locales = [undefined, undefined, 'zh-Hans-CN'];
document.querySelectorAll<CalendarSk>('calendar-sk').forEach((ele, i) => {
  ele.displayDate = new Date(2020, 4, 21);
  ele.locale = locales[i];
  ele.addEventListener('change', (e) => {
    evt.innerText = (e as CustomEvent<Date>).detail.toString();
  });
});

document.addEventListener('keydown', (e) => document.querySelector<CalendarSk>('calendar-sk')!.keyboardHandler(e));
