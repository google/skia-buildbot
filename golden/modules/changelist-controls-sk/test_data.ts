import { ChangeListSummaryResponse } from '../rpc_types';

export const twoPatchSets: ChangeListSummaryResponse = {
  cl: {
    system: 'gerrit',
    id: '1805837',
    owner: 'chromium-autoroll@example.google.com.iam.gserviceaccount.com',
    status: 'Open',
    subject: 'Roll src-internal da33810f35a7..af6fbc37d76b (1 commits)',
    updated: '2019-09-15T14:25:22Z',
    url: 'https://chromium-review.googlesource.com/1805837',
  },
  patch_sets: [
    {
      id: 'bd92c1d223172fe846fdd8f0fa6532ec2cd2ed72',
      order: 1,
      try_jobs: [
        {
          id: '8102241932564492368',
          name: 'android-nougat-arm64-rel',
          updated: '2019-09-15T13:25:32.686534Z',
          system: 'buildbucket',
          url: 'https://cr-buildbucket.appspot.com/build/8102241932564492368',
        },
      ],
    },
    {
      id: '0d88927361c931267cfa152c6c0ac87bd3e9a1c7',
      order: 4,
      try_jobs: [
        {
          id: '8902241932564492368',
          name: 'android-marshmallow-arm64-rel',
          updated: '2019-09-15T14:25:32.686534Z',
          system: 'buildbucket',
          url: 'https://cr-buildbucket.appspot.com/build/8902241932564492368',
        },
        {
          id: '8902241932564492048',
          name: 'linux-rel',
          updated: '2019-09-15T14:25:32.686534Z',
          system: 'buildbucket',
          url: 'https://cr-buildbucket.appspot.com/build/8902241932564492048',
        },
        {
          id: '8902241932564492512',
          name: 'mac-rel',
          updated: '2019-09-15T14:25:32.686534Z',
          system: 'buildbucket',
          url: 'https://cr-buildbucket.appspot.com/build/8902241932564492512',
        },
        {
          id: '8902241932564492144',
          name: 'win10_chromium_x64_rel_ng',
          updated: '2019-09-15T14:25:32.686534Z',
          system: 'buildbucket',
          url: 'https://cr-buildbucket.appspot.com/build/8902241932564492144',
        },
      ],
    },
  ],
  num_total_patch_sets: 2,
};
