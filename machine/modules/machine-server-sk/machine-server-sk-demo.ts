import './index';

import fetchMock from 'fetch-mock';
import {descriptions, fakeNow} from './demo_data';
import {MachineServerSk} from './machine-server-sk';

Date.now = () => fakeNow;

fetchMock.get('/_/machines', descriptions);

document.body.appendChild(new MachineServerSk());
