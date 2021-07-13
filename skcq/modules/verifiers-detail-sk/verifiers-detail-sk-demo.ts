import './index';

import fetchMock from 'fetch-mock';
import { GetChangeAttemptsResponse, GetChangeAttemptsRequest } from '../json';

const MockRequestWithData: GetChangeAttemptsRequest = {
  change_id: 123,
  patchset_id: 5,
};

const MockRequestWithNoData: GetChangeAttemptsRequest = {
  change_id: 345,
  patchset_id: 1,
};

const MockResponseWithData: GetChangeAttemptsResponse = {
  change_attempts: {
    attempts: [
      {
        change_id: 123,
        patchset_id: 5,
        dry_run: false,
        repo: 'skia',
        branch: 'main',
        start_ts: 0,
        stop_ts: 500,
        committed_ts: 0,
        cq_abandoned: false,
        submittable_changes: [],
        verifiers_statuses: [
          {
            name: 'TreeStatusVerifier',
            start_ts: 0,
            stop_ts: 300,
            reason: 'Tree is open.',
            state: 'SUCCESSFUL',
          },
          {
            name: 'ApprovedVerifier',
            start_ts: 0,
            stop_ts: 400,
            reason: 'Missing CQ+2 vote by a committer',
            state: 'FAILURE',
          },
        ],
        overall_status: 'FAILURE',
      },
      {
        change_id: 123,
        patchset_id: 5,
        dry_run: true,
        repo: 'skia',
        branch: 'main',
        start_ts: 0,
        stop_ts: 1000,
        committed_ts: 0,
        cq_abandoned: false,
        submittable_changes: ['434', '535'],
        verifiers_statuses: [
          {
            name: 'TreeStatusVerifier',
            start_ts: 200,
            stop_ts: 200,
            reason: 'Tree is in caution state. Waiting for it to open.',
            state: 'WAITING',
          },
          {
            name: 'DryRunAccessListVerifier',
            start_ts: 0,
            stop_ts: 900,
            reason: 'CQ+1 voted by allowed dry-run voters: batman@gotham.com',
            state: 'SUCCESSFUL',
          },
        ],
        overall_status: 'WAITING',
      },
    ],
  },
};

fetchMock.config.overwriteRoutes = false;
fetchMock.post('/_/get_change_attempts', MockResponseWithData, { body: MockRequestWithData });
fetchMock.post('/_/get_change_attempts', {}, { body: MockRequestWithNoData });

customElements.whenDefined('verifiers-detail-sk').then(() => {
  const pageWithData = document.createElement('verifiers-detail-sk');
  pageWithData.setAttribute('change_id', '123');
  pageWithData.setAttribute('patchset_id', '5');

  const pageNoData = document.createElement('verifiers-detail-sk');
  pageNoData.setAttribute('change_id', '345');
  pageNoData.setAttribute('patchset_id', '1');

  document.querySelector('h1')!
    .insertAdjacentElement('afterend', pageWithData)!
    .insertAdjacentElement('afterend', pageNoData);
});
