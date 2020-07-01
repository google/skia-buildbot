import './index.ts';
import {CalendarInputSk} from './calendar-input-sk';

const evt = document.getElementById('evt')!;
const locales = [undefined, undefined, undefined, 'zh-Hans-CN'];
document
  .querySelectorAll<CalendarInputSk>('calendar-input-sk')
  .forEach((ele, i) => {
    ele.displayDate = new Date(2020, 4, 21);
    ele.locale = locales[i];
    ele.addEventListener('change', (e) => {
      evt.innerText = (e as CustomEvent<Date>).detail.toString();
    });
  });

document
  .querySelector('#invalid')!
  .querySelector<HTMLInputElement>('input')!.value = '2020-';
