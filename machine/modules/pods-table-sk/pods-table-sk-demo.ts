import fetchMock from 'fetch-mock';

import { PodsTableSk } from './pods-table-sk';

fetchMock.get('/_/pods', [{ Name: 'switch-pod-0', LastUpdated: '2001-02-03T04:05:06.709012Z' },
  { Name: 'switch-pod-1', LastUpdated: '2002-03-04T05:06:07.890123Z' },
  { Name: 'switch-pod-2', LastUpdated: '2003-04-05T06:07:08.901234Z' }]);

const element = new PodsTableSk();
document.body.appendChild(element);
element.update();
