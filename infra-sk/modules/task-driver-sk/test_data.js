export const taskDriverData = {
  id: '20181022T152846.098672800Z_000000000095d900',
  name: 'Infra-Experimental-Small',
  isInfra: false,
  properties: {
    local: false,
    swarmingBot: 'skia-gce-212',
    swarmingServer: 'https://chromium-swarm.appspot.com',
    swarmingTask: '40e6ad0046722511',
  },
  result: 'FAILURE',
  started: '2018-10-22T15:29:49.642134239Z',
  finished: '2018-10-22T15:37:56.679300433Z',
  steps: [
    {
      id: '9920ca35-af15-4270-945e-f1c4434d0156',
      name: 'MkdirAll /mnt/pd0/s/w/ir/go_deps/src',
      isInfra: true,
      parent: 'root',
      result: 'SUCCESS',
      started: '2018-10-22T15:29:49.642421645Z',
      finished: '2018-10-22T15:29:49.642589276Z',
    },
    {
      id: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
      name: 'Ensure Git Checkout',
      isInfra: true,
      parent: 'root',
      result: 'SUCCESS',
      started: '2018-10-22T15:29:49.642668878Z',
      finished: '2018-10-22T15:30:42.643474759Z',
      steps: [
        {
          id: '5bf74179-e6f5-4e7e-918c-55e604f365c1',
          name: 'Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:29:49.642832485Z',
          finished: '2018-10-22T15:29:49.642928474Z',
        },
        {
          id: 'ec70d08e-eefe-4844-957e-3d11f3c2b4ca',
          name: 'Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra/.git',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:29:49.642992588Z',
          finished: '2018-10-22T15:29:49.643099032Z',
        },
        {
          id: 'f4c22bf4-2f5f-4406-bec9-07672c9bc062',
          name: 'git status',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:29:49.643240791Z',
          finished: '2018-10-22T15:30:40.886520331Z',
          data: [
            {
              type: 'log',
              data: {
                id: '0cba872b-9be6-4b39-af40-e146fae680ac',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'command',
              data: {
                command: ['git', 'status'],
              },
            },
            {
              type: 'log',
              data: {
                id: '582dc226-e93a-443b-9166-edc9f4af30e9',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
        },
        {
          id: '6d403ac7-f2e1-4b53-aeed-e59a6e98e77a',
          name: 'git remote -v',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:40.886747414Z',
          finished: '2018-10-22T15:30:40.893461626Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'remote', '-v'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'b97cf347-d44b-4ea3-8b12-27e5521c6983',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'log',
              data: {
                id: '444d2108-2fa9-4ba6-97ae-999544b28380',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
        },
        {
          id: 'd06643f9-c58c-4bd6-9f45-a9f70822eaf8',
          name: 'git rev-parse HEAD',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:40.893660164Z',
          finished: '2018-10-22T15:30:40.900764087Z',
          data: [
            {
              type: 'log',
              data: {
                id: '6155df3f-be84-4065-a495-504a399d84d9',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'command',
              data: {
                command: ['git', 'rev-parse', 'HEAD'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'ad071662-d6c5-4d77-a27e-6c1c11a6b128',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: '064ec8f1-4922-4a2c-b6a9-1609699ec5e8',
          name: 'Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:40.900923612Z',
          finished: '2018-10-22T15:30:40.901069569Z',
        },
        {
          id: '6e271921-f06d-4565-9ef1-a4678634d235',
          name: 'git fetch --prune origin',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:40.901215935Z',
          finished: '2018-10-22T15:30:41.540773069Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'fetch', '--prune', 'origin'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'a165d121-1ffa-43ee-8ba7-79481b4d3118',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: 'f21e0a1c-b550-4f9d-a6c5-fc5e1996500d',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: '21283080-d788-40ab-a154-edc299a58f65',
          name: 'git reset --hard HEAD',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:41.540956572Z',
          finished: '2018-10-22T15:30:41.577243835Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'reset', '--hard', 'HEAD'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'b45d6f2c-d391-4cf4-861f-d2b679a40f05',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: '5f20d8a1-6aa8-4e0e-b2e3-7c5e5024498b',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: '7d2892b7-ad13-4718-94fe-4cdd3c351ec0',
          name: 'git clean -d -f',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:41.577396173Z',
          finished: '2018-10-22T15:30:41.599060099Z',
          data: [
            {
              type: 'log',
              data: {
                id: '101ac719-0d2b-4836-9b76-383b0a137c4e',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'command',
              data: {
                command: ['git', 'clean', '-d', '-f'],
              },
            },
            {
              type: 'log',
              data: {
                id: '61455efc-7b98-4fe9-92e1-3fcfa2608bca',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
        },
        {
          id: 'c1892a55-b4ff-4165-b527-60085653c5bd',
          name: 'git checkout master -f',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:41.599238817Z',
          finished: '2018-10-22T15:30:41.629837365Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'checkout', 'master', '-f'],
              },
            },
            {
              type: 'log',
              data: {
                id: '72c97129-d811-4a67-bed9-07c42052fea3',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: '899cd69c-9e84-4bbb-a1d7-923af679c379',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: '0aa321dd-6b2f-45c5-80fd-cd11c742ea3e',
          name: 'git reset --hard origin/master',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:41.629985586Z',
          finished: '2018-10-22T15:30:41.667533217Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'reset', '--hard', 'origin/master'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'd64ec2b3-7675-493f-93ca-015e05acf9c3',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: '890a6487-041b-459e-94fc-edb54c2e2db1',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: 'a97ddfcb-34e2-4b5e-89e7-d0aa273905ea',
          name: 'git fetch https://skia.googlesource.com/buildbot refs/changes/07/161107/14',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:41.667770677Z',
          finished: '2018-10-22T15:30:42.330263346Z',
          data: [
            {
              type: 'log',
              data: {
                id: '41ac1517-63c3-4247-9b18-cdc57e8d8b11',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'command',
              data: {
                command: [
                  'git',
                  'fetch',
                  'https://skia.googlesource.com/buildbot',
                  'refs/changes/07/161107/14',
                ],
              },
            },
            {
              type: 'log',
              data: {
                id: 'fb0eaa15-af06-4b6b-b33d-254cbbf7a856',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
        {
          id: '07e8c8c8-61d8-4702-8b32-2b607babd0b8',
          name: 'git reset --hard FETCH_HEAD',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:42.330431628Z',
          finished: '2018-10-22T15:30:42.371148365Z',
          data: [
            {
              type: 'log',
              data: {
                id: '6334ee1f-9b30-4ac2-a36a-97eea29e1cad',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'command',
              data: {
                command: ['git', 'reset', '--hard', 'FETCH_HEAD'],
              },
            },
            {
              type: 'log',
              data: {
                id: '12e5b98f-4671-405b-947c-d740c81d579f',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
        },
        {
          id: '647670cf-85e5-4359-b612-269e65f73a0c',
          name: 'git rebase b173bcd1ba3215f3d8aa7384c0b2100c565dd458',
          isInfra: true,
          parent: 'f7bc5c4f-1bf5-493b-b8e4-fa288df1d949',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:42.371304882Z',
          finished: '2018-10-22T15:30:42.643285716Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['git', 'rebase', 'b173bcd1ba3215f3d8aa7384c0b2100c565dd458'],
              },
            },
            {
              type: 'log',
              data: {
                id: 'bbe2344b-0f55-4dc0-91aa-f38284fa4cc8',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: '4de154e6-c6ec-4b27-a00e-476fb82baffe',
                name: 'stdout',
                severity: 'INFO',
              },
            },
          ],
        },
      ],
    },
    {
      id: 'b452b069-05f1-48a0-a6d1-3e90900063d9',
      name: 'Set Go Environment',
      isInfra: false,
      environment: [
        'CHROME_HEADLESS=1',
        'GOROOT=/mnt/pd0/s/w/ir/go/go',
        'GOPATH=/mnt/pd0/s/w/ir/go_deps',
        'GIT_USER_AGENT=git/1.9.1',
        'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
        'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
        'TMPDIR=',
      ],
      parent: 'root',
      result: 'FAILURE',
      started: '2018-10-22T15:30:42.643590864Z',
      finished: '2018-10-22T15:37:56.67921026Z',
      steps: [
        {
          id: '17d96922-9364-4959-adb2-15e797d2af9b',
          name: 'which go',
          isInfra: false,
          environment: [
            'CHROME_HEADLESS=1',
            'GOROOT=/mnt/pd0/s/w/ir/go/go',
            'GOPATH=/mnt/pd0/s/w/ir/go_deps',
            'GIT_USER_AGENT=git/1.9.1',
            'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
            'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
            'TMPDIR=',
          ],
          parent: 'b452b069-05f1-48a0-a6d1-3e90900063d9',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:42.643792411Z',
          finished: '2018-10-22T15:30:42.645423984Z',
          data: [
            {
              type: 'log',
              data: {
                id: 'd5b03bcb-3292-4689-a642-11013c544400',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'log',
              data: {
                id: '458e4ffb-57d1-428d-b8ce-c890d1187b00',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'command',
              data: {
                command: ['which', 'go'],
                env: [
                  'CHROME_HEADLESS=1',
                  'GOROOT=/mnt/pd0/s/w/ir/go/go',
                  'GOPATH=/mnt/pd0/s/w/ir/go_deps',
                  'GIT_USER_AGENT=git/1.9.1',
                  'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
                  'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
                  'TMPDIR=',
                ],
              },
            },
          ],
        },
        {
          id: '8a304c68-06b0-469b-b86b-479392a9b583',
          name: 'go version',
          isInfra: false,
          environment: [
            'CHROME_HEADLESS=1',
            'GOROOT=/mnt/pd0/s/w/ir/go/go',
            'GOPATH=/mnt/pd0/s/w/ir/go_deps',
            'GIT_USER_AGENT=git/1.9.1',
            'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
            'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
            'TMPDIR=',
          ],
          parent: 'b452b069-05f1-48a0-a6d1-3e90900063d9',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:42.645630558Z',
          finished: '2018-10-22T15:30:42.658131885Z',
          data: [
            {
              type: 'log',
              data: {
                id: '66c9691e-9500-47b0-8b12-7c638c78b591',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
            {
              type: 'log',
              data: {
                id: 'bc7d90a3-c8ec-47a3-b8fa-94fbf631907a',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'command',
              data: {
                command: ['go', 'version'],
                env: [
                  'CHROME_HEADLESS=1',
                  'GOROOT=/mnt/pd0/s/w/ir/go/go',
                  'GOPATH=/mnt/pd0/s/w/ir/go_deps',
                  'GIT_USER_AGENT=git/1.9.1',
                  'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
                  'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
                  'TMPDIR=',
                ],
              },
            },
          ],
        },
        {
          id: '97ec7ef0-aec8-457f-8de4-663122469b52',
          name: 'sudo npm i -g bower@1.8.2',
          isInfra: false,
          environment: [
            'CHROME_HEADLESS=1',
            'GOROOT=/mnt/pd0/s/w/ir/go/go',
            'GOPATH=/mnt/pd0/s/w/ir/go_deps',
            'GIT_USER_AGENT=git/1.9.1',
            'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
            'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
            'TMPDIR=',
          ],
          parent: 'b452b069-05f1-48a0-a6d1-3e90900063d9',
          result: 'SUCCESS',
          started: '2018-10-22T15:30:42.658380456Z',
          finished: '2018-10-22T15:30:55.427302726Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['sudo', 'npm', 'i', '-g', 'bower@1.8.2'],
                env: [
                  'CHROME_HEADLESS=1',
                  'GOROOT=/mnt/pd0/s/w/ir/go/go',
                  'GOPATH=/mnt/pd0/s/w/ir/go_deps',
                  'GIT_USER_AGENT=git/1.9.1',
                  'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
                  'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
                  'TMPDIR=',
                ],
              },
            },
            {
              type: 'log',
              data: {
                id: 'f0137b7a-9e43-4653-b93a-56aa1e75311f',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'log',
              data: {
                id: '4338d64d-026f-41fc-92c7-9038e8b0116f',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
        },
        {
          id: 'e2d59319-dd03-42a9-a9de-384db4e9a9b1',
          name: 'go run ./run_unittests.go --alsologtostderr --small',
          isInfra: false,
          environment: [
            'CHROME_HEADLESS=1',
            'GOROOT=/mnt/pd0/s/w/ir/go/go',
            'GOPATH=/mnt/pd0/s/w/ir/go_deps',
            'GIT_USER_AGENT=git/1.9.1',
            'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
            'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
            'TMPDIR=',
          ],
          parent: 'b452b069-05f1-48a0-a6d1-3e90900063d9',
          result: 'FAILURE',
          started: '2018-10-22T15:30:55.427518602Z',
          finished: '2018-10-22T15:37:56.678996747Z',
          data: [
            {
              type: 'command',
              data: {
                command: ['go', 'run', './run_unittests.go', '--alsologtostderr', '--small'],
                env: [
                  'CHROME_HEADLESS=1',
                  'GOROOT=/mnt/pd0/s/w/ir/go/go',
                  'GOPATH=/mnt/pd0/s/w/ir/go_deps',
                  'GIT_USER_AGENT=git/1.9.1',
                  'PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools',
                  'SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools',
                  'TMPDIR=',
                ],
              },
            },
            {
              type: 'log',
              data: {
                id: '6c0ca473-39ab-4d58-b0ce-bd068d82ba40',
                name: 'stdout',
                severity: 'INFO',
              },
            },
            {
              type: 'log',
              data: {
                id: 'bf138d02-efcd-4dd0-877f-23ff3c17ed66',
                name: 'stderr',
                severity: 'ERROR',
              },
            },
          ],
          errors: [
            'Command exited with exit status 1: CHROME_HEADLESS=1 GIT_USER_AGENT=git/1.9.1 GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache GOFLAGS= GOPATH=/mnt/pd0/s/w/ir/gopath GOROOT=/mnt/pd0/s/w/ir/go/go PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/gopath/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin go generate ./...',
          ],
        },
      ],
    },
  ],
};
