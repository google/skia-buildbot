export const now = Date.parse('2020-03-22T00:00:00.000Z');

const mod2Data = Array(200).fill(1).map((_, index) => index % 2);
const mod3Data = Array(200).fill(1).map((_, index) => index % 3);

export const digestDetails = {
  test: 'ignores-page-sk_create-modal',
  digest: '7e4d902146ada7263369453982895fb9',
  status: 'positive',
  triage_history: [
    {
      user: 'user1@example.com',
      ts: '2020-02-25T16:08:18.776Z',
    },
    {
      user: 'user2@example.com',
      ts: '2020-02-25T16:05:13.000Z',
    },
  ],
  paramset: {
    ext: [
      'png',
    ],
    name: [
      'ignores-page-sk_create-modal',
    ],
    source_type: [
      'infra',
    ],
    os: [
      'Mac', 'Linux',
    ],
  },
  traces: {
    tileSize: 200,
    traces: [
      {
        data: mod2Data,
        label: ',name=ignores-page-sk_create-modal,os=Linux,source_type=infra,',
        params: {
          ext: 'png',
          name: 'ignores-page-sk_create-modal',
          os: 'Linux',
          source_type: 'infra',
        },
        comment_indices: null, // TODO(kjlubick) skbug.com/6630
      },
      {
        data: mod3Data,
        label: ',name=ignores-page-sk_create-modal,os=Mac,source_type=infra,',
        params: {
          ext: 'png',
          name: 'ignores-page-sk_create-modal',
          os: 'Mac',
          source_type: 'infra',
        },
        comment_indices: null,
      },
    ],
    digests: [
      {
        digest: '7e4d902146ada7263369453982895fb9',
        status: 'positive',
      },
      {
        digest: '9a58431879d4a9153f1c5e2f2cc62bb5',
        status: 'positive',
      },
      {
        digest: '6f8dcb2479012916146886d0c0fa8881',
        status: 'positive',
      },
    ],
    total_digests: 3,
  },
  closestRef: 'pos',
  refDiffs: {
    neg: null,
    pos: {
      numDiffPixels: 3766,
      pixelDiffPercent: 0.22284023,
      maxRGBADiffs: [
        9,
        9,
        9,
        0,
      ],
      dimDiffer: false,
      diffs: {
        combined: 0.082530275,
        percent: 0.22284023,
        pixel: 3766,
      },
      digest: '9a58431879d4a9153f1c5e2f2cc62bb5',
      status: 'positive',
      paramset: {
        ext: [
          'png',
        ],
        name: [
          'ignores-page-sk_create-modal',
        ],
        source_type: [
          'infra',
        ],
        os: [
          'Mac', 'Linux',
        ],
      },
      n: 143,
    },
  },
};
