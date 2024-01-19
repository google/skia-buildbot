import fetchMock from 'fetch-mock';
import { Status } from '../../../infra-sk/modules/json';

import './index';

const status: Status = {
  email: 'user@example.com',
  roles: ['admin'],
};

fetchMock.get('/_/login/status', status);
fetchMock.get('/_/configs', [
  {
    id: 'depot_tools',
    domain: 'Source',
    display_name: 'depot_tools',
    description:
      'A package of scripts to manage checkouts and code reviews. The depot_tools package includes gclient, gcl, git-cl, repo, and others.',
    phase: 'GA',
    teams_id: '1201906511198',
    code_path: [
      'https://chromium.googlesource.com/chromium/tools/depot_tools/+/refs/heads/main',
    ],
    audience: ['Chrome', 'PEEPSI'],
    adoption_stage: 'All',
    landing_page:
      'https://chromium.googlesource.com/chromium/tools/depot_tools/+/HEAD/README.md',
    docs: {
      'git cl':
        'https://chromium.googlesource.com/chromium/tools/depot_tools/+/HEAD/README.git-cl.md',
      gclient:
        'https://chromium.googlesource.com/chromium/tools/depot_tools/+/HEAD/README.gclient.md',
    },
    feedback: {
      Bugs: 'https://bugs.chromium.org/p/chromium/issues/entry?components=Infra%3ESDK',
      Forum: 'https://groups.google.com/a/chromium.org/forum/#!forum/infra-dev',
    },
    resources: {},
  },
  {
    id: 'gerrit',
    domain: 'Source',
    display_name: 'Gerrit',
    description:
      "Gerrit is a web-based team code collaboration tool enabling software developers to review each other's code modifications using a Web browser.",
    phase: 'GA',
    teams_id: '8953724333',
    code_path: [],
    audience: ['Chrome', 'PEEPSI'],
    adoption_stage: 'All',
    landing_page: 'https://goto.google.com/cider-g',
    docs: {
      Overview:
        'https://gerrit-review.googlesource.com/Documentation/intro-gerrit-walkthrough.html',
    },
    feedback: {
      Help: 'http://go/git-help',
    },
    resources: {},
  },
]);

customElements.whenDefined('tools-sk').then(() => {
  // Insert the element later, which should given enough time for fetchMock to be in place.
  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', document.createElement('tools-sk'));
});
