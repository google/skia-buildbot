import fetchMock from 'fetch-mock';
import { descriptions, fakeNow } from '../machines-table-sk/demo_data';
import { MachineAppSk } from './machine-app-sk';

Date.now = () => fakeNow;
fetchMock.get('/_/machines', descriptions);

document.body.appendChild(new MachineAppSk());
