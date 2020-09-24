import './index';
import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';
import {
  typicalDetails, negativeOnly, noRefs, noTraces, twoHundredCommits, fakeNow,
} from './test_data';
import { setImageEndpointsForDemos } from '../common';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';

Date.now = () => fakeNow;
testOnlySetSettings({
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

setImageEndpointsForDemos();
let ele = document.createElement('digest-details-sk');
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
$$('#normal').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = negativeOnly;
ele.commits = twoHundredCommits;
$$('#negative_only').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = noRefs;
ele.commits = twoHundredCommits;
$$('#no_refs').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.changeListID = '12345';
ele.crs = 'gerrit';
$$('#changelist_id').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = typicalDetails;
ele.commits = twoHundredCommits;
ele.right = typicalDetails.refDiffs.neg;
$$('#right_overridden').appendChild(ele);

ele = document.createElement('digest-details-sk');
ele.details = noTraces;
ele.commits = twoHundredCommits;
$$('#no_traces').appendChild(ele);

ele = document.createElement('digest-details-sk');
const noParams = JSON.parse(JSON.stringify(noTraces));
noParams.paramset = {};
ele.details = noParams;
ele.commits = twoHundredCommits;
$$('#no_params').appendChild(ele);

document.addEventListener('triage', (e) => {
  $$('#event').textContent = `triage: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('show-commits', (e) => {
  $$('#event').textContent = `show-commits: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('zoom-dialog-opened', (e) => {
  $$('#event').textContent = `zoom-dialog-opened: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('zoom-dialog-closed', (e) => {
  $$('#event').textContent = `zoom-dialog-closed: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('fetch-error', (e) => {
  $$('#event').textContent = `fetch-error: ${JSON.stringify(e.detail)}`;
});

fetchMock.post('/json/v1/triage', delay(() => {
  if ($$('#simulate-not-logged-in').checked) {
    return 403;
  }
  return 200;
}, 300));
