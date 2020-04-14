import './index';
import { fetchMock } from 'fetch-mock';
import {
  typicalDetails, negativeOnly, noRefs, noTraces, twoHundredCommits, fakeNow,
} from './test_data';
import { $$ } from '../../../common-sk/modules/dom';
import { setImageEndpointsForDemos } from '../common';
import { delay } from '../demo_util';

Date.now = () => fakeNow;

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
ele.issue = '12345';
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

document.addEventListener('triage', (e) => {
  $$('#event').textContent = `triage: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('show-commits', (e) => {
  $$('#event').textContent = `show-commits: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('zoom-clicked', (e) => {
  $$('#event').textContent = `zoom-clicked: ${JSON.stringify(e.detail)}`;
});
document.addEventListener('fetch-error', (e) => {
  $$('#event').textContent = `fetch-error: ${JSON.stringify(e.detail)}`;
});

fetchMock.post('/json/triage', delay(() => {
  if ($$('#simulate-not-logged-in').checked) {
    return 403;
  }
  return 200;
}, 300));
