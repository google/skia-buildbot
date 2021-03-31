import 'elements-sk/styles/buttons';

import { $$ } from 'common-sk/modules/dom';

import './index';
import {ConfirmDialogSk} from './confirm-dialog-sk';

$$('#ask')!.addEventListener('click', e => {
  $$<ConfirmDialogSk>('#dialog')!.open('Do something dangerous?').then(() => {
    $$('#results')!.textContent = 'Confirmed!';
  }).catch(() => {
    $$('#results')!.textContent = 'Cancelled!';
  });
})
