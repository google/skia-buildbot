import './index';
import { CommitDetailPickerSk } from './commit-detail-picker-sk';

const commitinfo = [
  {
    ts: 1439649751,
    author: 'foo (foo@example.org)',
    url: 'skia.googlesource.com/bar',
    message: 'Commit from foo.',
    hash: 'abcdef123',
    CommitID: {
      offset: 1,
    },
  },
  {
    ts: 1439648914,
    author: 'bar (bar@example.org)',
    url: 'skia.googlesource.com/foo',
    message: 'Commit from bar',
    hash: 'abcdef456',
    CommitID: {
      offset: 2,
    },
  },
  {
    ts: 1439649951,
    author: 'foo (foo@example.org)',
    url: 'https://codereview.chromium.org/1490543002',
    message: 'Whitespace change',
    hash: 'abcdef789',
    CommitID: {
      offset: 3,
    },
  },
  {
    ts: 1439699951,
    author: 'foo (foo@example.org)',
    url: 'https://codereview.chromium.org/1490543002',
    message: 'Another whitespace change',
    hash: 'abcdef101112',
    CommitID: {
      offset: 4,
    },
  },
];

const evt = document.querySelector('#evt')!;

document
  .querySelectorAll<CommitDetailPickerSk>('commit-detail-picker-sk')
  .forEach((panel) => {
    panel.details = commitinfo;
    panel.addEventListener('commit-selected', function (e: Event) {
      evt.textContent = JSON.stringify((e as CustomEvent).detail, null, '  ');
    });
  });
