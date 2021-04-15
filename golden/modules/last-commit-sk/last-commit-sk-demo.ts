import './index';

import fetchMock from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { $$ } from 'common-sk/modules/dom';
import { exampleStatusData } from './demo_data';

testOnlySetSettings({
  baseRepoURL: 'https://github.com/flutter/flutter',
});

fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// Now that the mock RPC is setup, create the element
const ele = document.createElement('last-commit-sk');
$$('#container')!.appendChild(ele);
