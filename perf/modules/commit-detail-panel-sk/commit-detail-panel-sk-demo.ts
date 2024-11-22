import './index';
import { CommitDetailPanelSk } from './commit-detail-panel-sk';
import { Commit, CommitNumber } from '../json';

Date.now = () => Date.parse('2020-03-22T00:00:00.000Z');

const commitinfo: Commit[] = [
  {
    ts: 1439649751,
    author: 'foo (foo@example.org)',
    url: 'skia.googlesource.com/bar',
    message: 'Commit from foo.',
    hash: 'abcdef123',
    offset: CommitNumber(1),
    body: 'Commit body.',
  },
  {
    ts: 1439648914,
    author: 'bar (bar@example.org)',
    url: 'skia.googlesource.com/foo',
    message: 'Commit from bar',
    hash: 'abcdef456',
    offset: CommitNumber(2),
    body: 'Commit body.',
  },
  {
    ts: 1439649951,
    author: 'foo (foo@example.org)',
    url: 'https://codereview.chromium.org/1490543002',
    message: 'Whitespace change',
    hash: 'abcdef789',
    offset: CommitNumber(3),
    body: 'Commit body.',
  },
  {
    ts: 1439699951,
    author: 'foo (foo@example.org)',
    url: 'https://codereview.chromium.org/1490543002',
    message: 'Another whitespace change',
    hash: 'abcdef101112',
    offset: CommitNumber(4),
    body: 'Commit body.',
  },
];

const evt = document.querySelector('#evt')!;

document.querySelectorAll<CommitDetailPanelSk>('commit-detail-panel-sk').forEach((panel) => {
  panel.details = commitinfo;
  panel.addEventListener('commit-selected', (e: Event) => {
    evt.textContent = JSON.stringify((e as CustomEvent).detail, null, '  ');
  });
});
