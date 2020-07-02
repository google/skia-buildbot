import './index';
import { DomainPickerSk } from './domain-picker-sk';

const begin = new Date(2020, 4, 1).valueOf() / 1000;
const end = new Date(2020, 5, 1).valueOf() / 1000;
document.querySelectorAll<DomainPickerSk>('domain-picker-sk').forEach((ele) => {
  (ele as DomainPickerSk).state = {
    begin,
    end,
    num_commits: 50,
    request_type: 0,
  };
});
