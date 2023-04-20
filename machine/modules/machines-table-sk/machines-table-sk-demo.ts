import './index';

import fetchMock from 'fetch-mock';
import { descriptions, fakeNow } from './demo_data';
import { MachinesTableSk } from './machines-table-sk';

Date.now = () => fakeNow;

fetchMock.get('/_/machines', descriptions);

(async () => {
  const element = new MachinesTableSk();
  document.body.appendChild(element);
  await element.update();

  element.querySelectorAll<HTMLDetailsElement>('details.dimensions')[1]!.open =
    true;
})();
