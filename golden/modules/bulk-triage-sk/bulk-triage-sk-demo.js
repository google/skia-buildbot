import './index';
import { $$ } from 'common-sk/modules/dom';
import { examplePageData, exampleAllData } from './test_data';
import { fetchMock } from 'fetch-mock';
const handleTriaged = () => {
  const log = $$('#event_log');
  log.textContent += 'Did triage.\n';
};

const handleCancelled = () => {
  const log = $$('#event_log');
  log.textContent += 'Cancelled.\n';
};

const ele = document.createElement('bulk-triage-sk');
ele.setDigests(examplePageData, exampleAllData);
ele.addEventListener('bulk_triage_invoked', handleTriaged);
ele.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#default').appendChild(ele);

const eleCL = document.createElement('bulk-triage-sk');
eleCL.setDigests(examplePageData, exampleAllData);
eleCL.changeListID = '1234567';
eleCL.crs = 'github';
eleCL.addEventListener('bulk_triage_invoked', handleTriaged);
ele.addEventListener('bulk_triage_cancelled', handleCancelled);
$$('#changelist').appendChild(eleCL);

fetchMock.post('/json/triage', 200);
