import './index';

import fetchMock from 'fetch-mock';
import { GetCurrentChangesResponse, GetCurrentChangesRequest } from '../json';

const DryRunRequest: GetCurrentChangesRequest = {
  is_dry_run: true,
};

const DryRunMockResponse: GetCurrentChangesResponse = {
  changes: [
    {
      change_id: 123,
      latest_patchset_id: 2,
      repo: 'skia',
      branch: 'main',
      start_ts: Date.now() / 1000,
      internal: false,
      change_subject: 'Batcave in Gotham',
      change_owner: 'Batman',
      dry_run: true,
    },
    {
      change_id: 155,
      latest_patchset_id: 1,
      repo: 'skia',
      branch: 'main',
      start_ts: Date.now() / 1000,
      internal: false,
      change_subject: 'Looking for the Batcave',
      change_owner: 'Joker',
      dry_run: true,
    },
  ],
};

const CQRunRequest: GetCurrentChangesRequest = {
  is_dry_run: false,
};

const CQRunMockResponse: GetCurrentChangesResponse = {
  changes: [
    {
      change_id: 431,
      latest_patchset_id: 5,
      repo: 'skia',
      branch: 'dev-branch',
      start_ts: Date.now() / 1000,
      internal: false,
      change_subject: 'up up and away',
      change_owner: 'Superman',
      dry_run: false,
    },
  ],
};

fetchMock.config.overwriteRoutes = false;
fetchMock.post('/_/get_current_changes', DryRunMockResponse, { body: DryRunRequest });
fetchMock.post('/_/get_current_changes', CQRunMockResponse, { body: CQRunRequest });

customElements.whenDefined('processing-table-sk').then(() => {
  const dryRunPage = document.createElement('processing-table-sk');
  dryRunPage.setAttribute('dryrun', 'true');

  const cqRunPage = document.createElement('processing-table-sk');

  document.querySelector('h1')!
    .insertAdjacentElement('afterend', dryRunPage)!
    .insertAdjacentElement('afterend', cqRunPage);
});
