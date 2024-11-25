import { deepCopy } from '../../../infra-sk/modules/object';
import { Commit, Digest, SearchResult, TestName } from '../rpc_types';

export const fakeNow = Date.parse('2020-03-22T00:00:00.000Z');

const allTheSame: number[] = Array(200).fill(0);
const mod2Data: number[] = Array(200)
  .fill(1)
  .map((_, index) => index % 2);
const mod3Data: number[] = Array(200)
  .fill(1)
  .map((_, index) => index % 3);

export const makeTypicalSearchResult = (
  testName: TestName,
  digest: Digest,
  closestDigest: Digest
): SearchResult => ({
  test: testName,
  digest: digest,
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
    ext: ['png'],
    name: [testName],
    source_type: ['infra'],
    os: ['Mac', 'Linux'],
  },
  traces: {
    traces: [
      {
        data: mod2Data,
        label: `,name=${testName},os=Linux,source_type=infra,`,
        params: {
          ext: 'png',
          name: testName,
          os: 'Linux',
          source_type: 'infra',
        },
        comment_indices: null, // TODO(kjlubick) skbug.com/6630
      },
      {
        data: mod3Data,
        label: `,name=${testName},os=Mac,source_type=infra,`,
        params: {
          ext: 'png',
          name: testName,
          os: 'Mac',
          source_type: 'infra',
        },
        comment_indices: null,
      },
    ],
    digests: [
      {
        digest: digest,
        status: 'positive',
      },
      {
        digest: closestDigest,
        status: 'positive',
      },
      {
        digest: 'ec3b8f27397d99581e06eaa46d6d5837',
        status: 'negative',
      },
    ],
    total_digests: 3,
  },
  closestRef: 'pos',
  refDiffs: {
    neg: {
      numDiffPixels: 1689996,
      pixelDiffPercent: 99.99976,
      maxRGBADiffs: [255, 255, 255, 0],
      dimDiffer: true,
      combinedMetric: 9.306038,
      digest: 'ec3b8f27397d99581e06eaa46d6d5837',
      status: 'negative',
      paramset: {
        ext: ['png'],
        name: [testName],
        source_type: ['infra'],
        os: ['Linux'],
      },
    },
    pos: {
      numDiffPixels: 3766,
      pixelDiffPercent: 0.22284023,
      maxRGBADiffs: [9, 9, 9, 0],
      dimDiffer: false,
      combinedMetric: 0.082530275,
      digest: closestDigest,
      status: 'positive',
      paramset: {
        ext: ['png'],
        name: [testName],
        source_type: ['infra'],
        os: ['Mac', 'Linux'],
      },
    },
  },
});

export const typicalDetails = makeTypicalSearchResult(
  'dots-legend-sk_too-many-digests',
  '6246b773851984c726cb2e1cb13510c2',
  '99c58c7002073346ff55f446d47d6311'
);

const disallowTriaging = (searchResult: SearchResult): SearchResult => {
  const retVal = deepCopy(searchResult);
  retVal.paramset.image_matching_algorithm = ['positive_if_only_digest'];
  retVal.paramset.disallow_triaging = ['true'];
  retVal.traces.traces![0].params!.image_matching_algorithm = 'positive_if_only_digest';
  retVal.traces.traces![0].params!.disallow_triaging = 'true';
  return retVal;
};

export const typicalDetailsDisallowTriaging = disallowTriaging(typicalDetails);

export const negativeOnly: SearchResult = {
  test: 'dots-legend-sk_too-many-digests',
  digest: '6246b773851984c726cb2e1cb13510c2',
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
    ext: ['png'],
    name: ['dots-legend-sk_too-many-digests'],
    source_type: ['infra'],
    os: ['Mac', 'Linux'],
  },
  traces: {
    traces: [
      {
        data: mod2Data,
        label: ',name=dots-legend-sk_too-many-digests,os=Linux,source_type=infra,',
        params: {
          ext: 'png',
          name: 'dots-legend-sk_too-many-digests',
          os: 'Linux',
          source_type: 'infra',
        },
        comment_indices: null,
      },
    ],
    digests: [
      {
        digest: '6246b773851984c726cb2e1cb13510c2',
        status: 'positive',
      },
      {
        digest: 'ec3b8f27397d99581e06eaa46d6d5837',
        status: 'negative',
      },
    ],
    total_digests: 3,
  },
  closestRef: 'neg',
  refDiffs: {
    neg: {
      numDiffPixels: 1689996,
      pixelDiffPercent: 99.99976,
      maxRGBADiffs: [255, 255, 255, 0],
      dimDiffer: true,
      combinedMetric: 9.306038,
      digest: 'ec3b8f27397d99581e06eaa46d6d5837',
      status: 'negative',
      paramset: {
        ext: ['png'],
        name: ['dots-legend-sk_too-many-digests'],
        source_type: ['infra'],
        os: ['Mac'],
      },
    },
  },
};

export const noRefs: SearchResult = {
  test: 'dots-legend-sk_too-many-digests',
  digest: '6246b773851984c726cb2e1cb13510c2',
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
    ext: ['png'],
    name: ['dots-legend-sk_too-many-digests'],
    source_type: ['infra'],
    os: ['Linux'],
  },
  traces: {
    traces: [
      {
        data: allTheSame,
        label: ',name=dots-legend-sk_too-many-digests,os=Linux,source_type=infra,',
        params: {
          ext: 'png',
          name: 'dots-legend-sk_too-many-digests',
          os: 'Linux',
          source_type: 'infra',
        },
        comment_indices: null,
      },
    ],
    digests: [
      {
        digest: '6246b773851984c726cb2e1cb13510c2',
        status: 'positive',
      },
    ],
    total_digests: 3,
  },
  closestRef: '',
  refDiffs: {},
};

export const noRefsDisallowTriaging = disallowTriaging(noRefs);

export const noRefsYet: SearchResult = {
  test: 'dots-legend-sk_too-many-digests',
  digest: '6246b773851984c726cb2e1cb13510c2',
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
    ext: ['png'],
    name: ['dots-legend-sk_too-many-digests'],
    source_type: ['infra'],
    os: ['Linux'],
  },
  traces: {
    traces: [
      {
        data: mod2Data,
        label: ',name=dots-legend-sk_too-many-digests,os=Linux,source_type=infra,',
        params: {
          ext: 'png',
          name: 'dots-legend-sk_too-many-digests',
          os: 'Linux',
          source_type: 'infra',
        },
        comment_indices: null,
      },
    ],
    digests: [
      {
        digest: '6246b773851984c726cb2e1cb13510c2',
        status: 'positive',
      },
      {
        digest: 'ec3b8f27397d99581e06eaa46d6d5837',
        status: 'negative',
      },
    ],
    total_digests: 3,
  },
  closestRef: '',
  refDiffs: {},
};

export const noTraces: SearchResult = {
  test: 'dots-legend-sk_too-many-digests',
  digest: '6246b773851984c726cb2e1cb13510c2',
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
  traces: {
    traces: [],
    digests: [],
    total_digests: 0,
  },
  paramset: {
    ext: ['png'],
    name: ['dots-legend-sk_too-many-digests'],
    source_type: ['infra'],
    os: ['Mac', 'Linux'],
  },
  closestRef: 'pos',
  refDiffs: {
    neg: {
      numDiffPixels: 1689996,
      pixelDiffPercent: 99.99976,
      maxRGBADiffs: [255, 255, 255, 0],
      dimDiffer: true,
      combinedMetric: 9.306038,
      digest: 'ec3b8f27397d99581e06eaa46d6d5837',
      status: 'negative',
      paramset: {
        ext: ['png'],
        name: ['dots-legend-sk_too-many-digests'],
        source_type: ['infra'],
        os: ['Linux'],
      },
    },
    pos: {
      numDiffPixels: 3766,
      pixelDiffPercent: 0.22284023,
      maxRGBADiffs: [9, 9, 9, 0],
      dimDiffer: false,
      combinedMetric: 0.082530275,
      digest: '99c58c7002073346ff55f446d47d6311',
      status: 'positive',
      paramset: {
        ext: ['png'],
        name: ['dots-legend-sk_too-many-digests'],
        source_type: ['infra'],
        os: ['Mac', 'Linux'],
      },
    },
  },
};

const tsStartTime = 1583130000; // an arbitrary timestamp.

function makeCommits(n: number): Commit[] {
  const rv: Commit[] = [];
  for (let i = 0; i < n; i++) {
    rv.push({
      id: 'commit_id',
      commit_time: tsStartTime + i * 123, // arbitrary spacing
      hash: `${i}`.padEnd(32, '0'), // make a deterministic "md5 hash", which is 32 chars long
      author: `user${i % 7}@example.com`,
      message: `This is a nice message. I've repeated it ${i + 1} time(s)`,
      cl_url: '',
    });
  }
  return rv;
}

export const twoHundredCommits = makeCommits(200);
