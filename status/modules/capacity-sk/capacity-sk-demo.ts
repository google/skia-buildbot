import { SetupMocks } from '../rpc-mock';
import './index';
import { resp } from './test-data';

SetupMocks().expectGetBotUsage(resp);
const el = document.createElement('capacity-sk');
document.querySelector('#container')?.appendChild(el);
