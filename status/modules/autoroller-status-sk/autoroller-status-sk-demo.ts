import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { getAutorollerStatusesResponse, SetupMocks } from '../rpc-mock';

SetupMocks().expectGetAutorollerStatuses(getAutorollerStatusesResponse);
const el = document.createElement('autoroller-status-sk');
document.querySelector('#container')?.appendChild(el);
