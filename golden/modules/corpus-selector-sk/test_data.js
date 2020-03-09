export const trstatus = {
  ok: false,
  firstCommit: {
    commit_time: 1572357082,
    hash: 'ee08d523f60a04499c9023a349ef8174ab301f8f',
    author: 'Alice (alice@example.com)',
  },
  lastCommit: {
    commit_time: 1573598625,
    hash: '9501212cd0580acfed85a90c3a16b81847fde482',
    author: 'Bob (bob@example.com)',
  },
  totalCommits: 256,
  filledCommits: 256,
  corpStatus: [{
    name: 'canvaskit',
    ok: false,
    minCommitHash: '',
    untriagedCount: 2,
    negativeCount: 2,
  }, {
    name: 'colorImage',
    ok: true,
    minCommitHash: '',
    untriagedCount: 0,
    negativeCount: 1,
  }, {
    name: 'gm',
    ok: false,
    minCommitHash: '',
    untriagedCount: 61,
    negativeCount: 1494,
  }, {
    name: 'image',
    ok: false,
    minCommitHash: '',
    untriagedCount: 22,
    negativeCount: 35,
  }, {
    name: 'pathkit',
    ok: true,
    minCommitHash: '',
    untriagedCount: 0,
    negativeCount: 0,
  }, {
    name: 'skp',
    ok: true,
    minCommitHash: '',
    untriagedCount: 0,
    negativeCount: 1,
  }, {
    name: 'svg',
    ok: false,
    minCommitHash: '',
    untriagedCount: 19,
    negativeCount: 21,
  }],
};
