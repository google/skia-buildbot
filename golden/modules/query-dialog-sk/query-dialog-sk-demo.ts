import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { QueryDialogSk } from './query-dialog-sk';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';

const queryDialogSk = new QueryDialogSk()
$$('gold-scaffold-sk')?.appendChild(queryDialogSk);

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};

$$<HTMLButtonElement>('#show-dialog')!.addEventListener('click', () => {
  queryDialogSk.open(paramSet, '');
});

$$<HTMLButtonElement>('#show-dialog-with-selection')!.addEventListener('click', () => {
  queryDialogSk.open(paramSet, fromParamSet({'car make': ['dodge', 'ford'], 'color': ['red']}));
});
