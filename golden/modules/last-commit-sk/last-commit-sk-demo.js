import './index';

import { fetchMock } from 'fetch-mock';
import { testOnlySetSettings } from '../settings';
import { $$ } from 'common-sk/modules/dom';

testOnlySetSettings({
  baseRepoURL: 'https://github.com/flutter/flutter',
});

const statusData = {
  ok: true,
  firstCommit: {
    commit_time: 1597819864,
    hash: 'ce63f507336fa86278204dfac2b9a9546f81ab96',
    author: 'Alpha Beta (alphabeta@example.com)',
    message: "Document how to size IV's child correctly, after seeing confusion in Github issues (#64100)",
    cl_url: '',
  },
  lastCommit: {
    commit_time: 1598983079,
    hash: 'a8281e31afa9dddfa0764f59128c3a2360c48f49',
    author: 'Foxtrot Delta (foxtrot.delta@example.com)',
    message: 'Mark large_image_changer tests as not flaky (#65033)',
    cl_url: '',
  },
  totalCommits: 200,
  filledCommits: 200,
  corpStatus: [{
    name: 'flutter', ok: true, minCommitHash: '', untriagedCount: 0, negativeCount: 0,
  }],
};

fetchMock.get('/json/trstatus', JSON.stringify(statusData));

// Now that the mock RPC is setup, create the element
const ele = document.createElement('last-commit-sk');
$$('#container').appendChild(ele);
