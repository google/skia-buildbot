export const chromiumRevResult = {
  commit: 'deadbeefdeadbeef',
  tree: 'badbeef',
  parents: [
    '123456789',
  ],
  author: {
    name: 'alice',
    email: 'alice@bob.org',
    time: 'Mon May 08 21:08:33 2017',
  },
  committer: {
    name: 'Commit bot',
    email: 'commit-bot@chromium.org',
    time: 'Mon May 08 21:08:33 2017',
  },
  message: 'Do a thing',
  tree_diff: [
    {
      type: 'modify',
      old_id: '123456',
      old_mode: 33188,
      old_path: 'some/path',
      new_id: '789123',
      new_mode: 33188,
      new_path: 'some/path',
    },
  ],
};

export const skiaRevResult = {
  commit: '123456789abcdef',
  tree: 'abcdef123456789',
  parents: [
    'deadbeef',
  ],
  author: {
    name: 'bob',
    email: 'bob@alice.org',
    time: 'Mon May 08 21:08:33 2017',
  },
  committer: {
    name: 'Commit bot',
    email: 'commit-bot@chromium.org',
    time: 'Mon May 08 21:08:33 2017',
  },
  message: 'Do a thing',
  tree_diff: [
    {
      type: 'modify',
      old_id: '123456',
      old_mode: 33188,
      old_path: 'some/path',
      new_id: '789123',
      new_mode: 33188,
      new_path: 'some/path',
    },
  ],
};
