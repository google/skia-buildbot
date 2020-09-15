import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../commits-data-sk';
import { $$ } from 'common-sk/modules/dom';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';

SetupMocks(mockIncrementalResponse);

const data = document.createElement('commits-data-sk');
($$('#container') as HTMLElement).appendChild(data);
const table = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(table);
