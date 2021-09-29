import { ClusterDiffResult, Digest } from '../rpc_types';

export const positiveDigest: Digest = '99c58c7002073346ff55f446d47d6311';
export const negativeDigest: Digest = 'ec3b8f27397d99581e06eaa46d6d5837';
export const untriagedDigest: Digest = '6246b773851984c726cb2e1cb13510c2';

// This is the data returned from Gold's /clusterdiff RPC. Not all of it is used in
// cluster-digests-sk.
export const clusterDiffJSON: ClusterDiffResult = {
  nodes: [
    {
      name: positiveDigest,
      status: 'positive',
    },
    {
      name: untriagedDigest,
      status: 'untriaged',
    },
    {
      name: negativeDigest,
      status: 'negative',
    },
  ],
  links: [
    {
      source: 0,
      target: 1,
      value: 2, // The first two images are pretty similar
    },
    {
      source: 0,
      target: 2,
      value: 10, // The third image is quite different from the other two
    },
    {
      source: 1,
      target: 2,
      value: 11, // The third image is quite different from the other two
    },
  ],
  test: 'dots-legend-sk_too-many-digests',
  paramsetByDigest: {
    [positiveDigest]: {
      ext: [
        'png',
      ],
      name: [
        'dots-legend-sk_too-many-digests',
      ],
      gpu: [
        'nVidia',
      ],
      source_type: [
        'infra',
      ],
    },
    [untriagedDigest]: {
      ext: [
        'png',
      ],
      name: [
        'dots-legend-sk_too-many-digests',
      ],
      gpu: [
        'AMD',
      ],
      source_type: [
        'infra',
      ],
    },
    [negativeDigest]: {
      ext: [
        'png',
      ],
      name: [
        'dots-legend-sk_too-many-digests',
      ],
      gpu: [
        'AMD', 'nVidia',
      ],
      source_type: [
        'infra',
      ],
    },
  },
  paramsetsUnion: {
    ext: [
      'png',
    ],
    name: [
      'dots-legend-sk_too-many-digests',
    ],
    gpu: [
      'AMD', 'nVidia',
    ],
    source_type: [
      'infra', 'some-other-corpus',
    ],
  },
};
