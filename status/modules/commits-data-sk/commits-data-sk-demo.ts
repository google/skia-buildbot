import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { SetupMocks } from '../rpc-mock';

SetupMocks();

const ele = document.createElement('commits-data-sk');
($$('#container') as HTMLElement).appendChild(ele);
