import { ByBlameResponse, StatusResponse } from '../rpc_types';

export const fakeNow = Date.parse('2019-11-08T00:00:00Z');

export const canvaskit: ByBlameResponse = {
  data: [],
};

export const gm: ByBlameResponse = {
  data: [
    {
      groupID:
        '4edb719f1bc49bae585ff270df17f08039a96b6c:252cdb782418949651cc5eb7d467c57ddff3d1c7:' +
        'a1050ed2b1120613d9ae9587e3c0f4116e17337f:3f7c865936cc808af26d88bc1f5740a29cfce200:' +
        '05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
      nDigests: 1,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'complexclip_bw_layer',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '457875b610908dde8bfc5f45c907eb62',
        },
      ],
      commits: [
        {
          id: 'elisa05f6',
          commit_time: 1573171214,
          hash: '05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
          author: 'Elisa (elisa@example.com)',
          message: 'Revert Revert Reland Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'iris3f7c',
          commit_time: 1573171074,
          hash: '3f7c865936cc808af26d88bc1f5740a29cfce200',
          author: 'Iris (iris@example.com)',
          message: 'Revert Reland Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'daniela105',
          commit_time: 1573170075,
          hash: 'a1050ed2b1120613d9ae9587e3c0f4116e17337f',
          author: 'Daniel (daniel@example.com)',
          message: 'Reland Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'elisa252c',
          commit_time: 1573179814,
          hash: '252cdb782418949651cc5eb7d467c57ddff3d1c7',
          author: 'Elisa (elisa@example.com)',
          message: 'Revert Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'joe4edb',
          commit_time: 1573179564,
          hash: '4edb719f1bc49bae585ff270df17f08039a96b6c',
          author: 'Joe (joe@example.com)',
          message: 'Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '4edb719f1bc49bae585ff270df17f08039a96b6c:252cdb782418949651cc5eb7d467c57ddff3d1c7',
      nDigests: 7,
      nTests: 7,
      affectedTests: [
        {
          grouping: {
            name: 'aarectmodes',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: 'c6476baec94eb6a5071606575318e4df',
        },
        {
          grouping: {
            name: 'aaxfermodes',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '4acfd6b3a3943cc5d75cd22e966ae6f1',
        },
        {
          grouping: {
            name: 'hairmodes',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: 'f9e20c63b5ce3b58d9b6a90fa3e7224c',
        },
        {
          grouping: {
            name: 'imagefilters_xfermodes',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '47644613317040264fea6fa815af32e8',
        },
        {
          grouping: {
            name: 'lattice2',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '16e41798ecd59b1523322a57b49cc17f',
        },
        {
          grouping: {
            name: 'xfermodes',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '8fbee03f794c455c4e4842ec2736b744',
        },
        {
          grouping: {
            name: 'xfermodes3',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: 'fed2ff29abe371fc0ec1b2c65dfb3949',
        },
      ],
      commits: [
        {
          id: 'elisa252c',
          commit_time: 1573149814,
          hash: '252cdb782418949651cc5eb7d467c57ddff3d1c7',
          author: 'Elisa (elisa@example.com)',
          message: 'Reland Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'joe4edb',
          commit_time: 1573149564,
          hash: '4edb719f1bc49bae585ff270df17f08039a96b6c',
          author: 'Joe (joe@example.com)',
          message: 'Revert Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '73a722ce97ad935f936a4c7512b6724c41e0ce4e',
      nDigests: 41,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'skottie_colorize',
            source_type: 'infra',
          },
          num: 41,
          sample_digest: '024ce342b014c6fdb000f7c18d6d0775',
        },
      ],
      commits: [
        {
          id: 'bob73a7',
          commit_time: 1572978684,
          hash: '73a722ce97ad935f936a4c7512b6724c41e0ce4e',
          author: 'Bob (bob@example.com)',
          message: 'Update commits to have subject information 4',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '85c3d68f2539ed7a1e71f6c9d12baaf9e6be59d8',
      nDigests: 51,
      nTests: 31,
      affectedTests: null,
      commits: [
        {
          id: 'alice85c3',
          commit_time: 1572899861,
          hash: '85c3d68f2539ed7a1e71f6c9d12baaf9e6be59d8',
          author: 'Alice (alice@example.com)',
          message: 'Update commits to have subject information 3',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '7da048b5e8f17374bcd5baf48539eaa7ebe40e5c',
      nDigests: 13,
      nTests: 3,
      affectedTests: [
        {
          grouping: {
            name: 'shadow_utils',
            source_type: 'infra',
          },
          num: 5,
          sample_digest: '134d3c4fd609cd7f2e9cca43d78aa5d3',
        },
        {
          grouping: {
            name: 'shadow_utils_gray',
            source_type: 'infra',
          },
          num: 4,
          sample_digest: '292eb1e5b5860ba278ffa73efc9dd7c1',
        },
        {
          grouping: {
            name: 'shadow_utils_occl',
            source_type: 'infra',
          },
          num: 4,
          sample_digest: '918df0cc65d1b3ae7b8d8041afa40635',
        },
      ],
      commits: [
        {
          id: 'frank7da0',
          commit_time: 1572442816,
          hash: '7da048b5e8f17374bcd5baf48539eaa7ebe40e5c',
          author: 'Frank (frank@example.com)',
          message: 'Update commits to have subject information 2',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '342fbc54844d0d3fc9d20e20b45115db1e33395b',
      nDigests: 1,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'dftext_blob_persp',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '42a7bfc2d825412dfb82f6e3a12106c2',
        },
      ],
      commits: [
        {
          id: 'liam342f',
          commit_time: 1571959019,
          hash: '342fbc54844d0d3fc9d20e20b45115db1e33395b',
          author: 'Liam (liam@example.com)',
          message:
            'Oh my goodness, this is an incredibly long subject line. ' +
            'How will it every fit on the page? I hope this draws ok. Some webpages ' +
            'do not work when the content is way bigger than anticipated. I hope this is ' +
            'not one of those websites',
          cl_url: '',
        },
      ],
    },
  ],
};

export const svg: ByBlameResponse = {
  data: [
    {
      groupID: 'd2c67f44f8c2351e60e6ee224a060e916cd44f34',
      nDigests: 12,
      nTests: 12,
      affectedTests: null,
      commits: [
        {
          id: 'alfa01c6',
          commit_time: 1571948193,
          hash: '01c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'alfa (alfa@example.com)',
          message: 'Commit #1',
          cl_url: '',
        },
        {
          id: 'bravo02c6',
          commit_time: 1571948193,
          hash: '02c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'bravo (bravo@example.com)',
          message: 'Commit #2',
          cl_url: '',
        },
        {
          id: 'charlie03c6',
          commit_time: 1571948193,
          hash: '03c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'charlie (charlie@example.com)',
          message: 'Commit #3',
          cl_url: '',
        },
        {
          id: 'delta04c6',
          commit_time: 1571948193,
          hash: '04c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'delta (delta@example.com)',
          message: 'Commit #4',
          cl_url: '',
        },
        {
          id: 'echo05c6',
          commit_time: 1571948193,
          hash: '05c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'echo (echo@example.com)',
          message: 'Commit #5',
          cl_url: '',
        },
        {
          id: 'foxtrot06c6',
          commit_time: 1571948193,
          hash: '06c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'foxtrot (foxtrot@example.com)',
          message: 'Commit #6',
          cl_url: '',
        },
        {
          id: 'golf07c6',
          commit_time: 1571948193,
          hash: '07c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'golf (golf@example.com)',
          message: 'Commit #7',
          cl_url: '',
        },
        {
          id: 'hotel08c6',
          commit_time: 1571948193,
          hash: '08c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'hotel (hotel@example.com)',
          message: 'Commit #8',
          cl_url: '',
        },
        {
          id: 'india09c6',
          commit_time: 1571948193,
          hash: '09c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'india (india@example.com)',
          message: 'Commit #9',
          cl_url: '',
        },
        {
          id: 'juliett10c6',
          commit_time: 1571948193,
          hash: '10c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'juliett (juliett@example.com)',
          message: 'Commit #10',
          cl_url: '',
        },
        {
          id: 'kilo11c6',
          commit_time: 1571948193,
          hash: '11c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'kilo (kilo@example.com)',
          message: 'Commit #11',
          cl_url: '',
        },
        {
          id: 'example12c6',
          commit_time: 1571948193,
          hash: '12c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'lima (lima@example.com)',
          message: 'Commit #12',
          cl_url: '',
        },
        {
          id: 'mike13c6',
          commit_time: 1571948193,
          hash: '13c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'mike (mike@example.com)',
          message: 'Commit #13',
          cl_url: '',
        },
        {
          id: 'oscar14c6',
          commit_time: 1571948193,
          hash: '14c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'oscar (oscar@example.com)',
          message: 'Commit #14',
          cl_url: '',
        },
        {
          id: 'papa15c6',
          commit_time: 1571948193,
          hash: '15c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'papa (papa@example.com)',
          message: 'Commit #15',
          cl_url: '',
        },
        {
          id: 'quebec16c6',
          commit_time: 1571948193,
          hash: '16c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'quebec (quebec@example.com)',
          message: 'Commit #16',
          cl_url: '',
        },
        {
          id: 'romeo17c6',
          commit_time: 1571948193,
          hash: '17c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'romeo (romeo@example.com)',
          message: 'Commit #17',
          cl_url: '',
        },
        {
          id: 'sierra18c6',
          commit_time: 1571948193,
          hash: '18c67f44f8c2351e60e6ee224a060e916cd44f34',
          author: 'sierra (sierra@example.com)',
          message: 'Commit #18',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
      nDigests: 2,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'A_large_blank_world_map_with_oceans_marked_in_blue.svg',
            source_type: 'infra',
          },
          num: 2,
          sample_digest: '3c62b1dd009bb18bbb84d862cbc3652c',
        },
      ],
      commits: [
        {
          id: 'elisa05f6',
          commit_time: 1573151114,
          hash: '05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
          author: 'Elisa (elisa@example.com)',
          message: 'Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
    {
      groupID: 'd84dd4babb71796ee194fa1913150d86d6aa643b',
      nDigests: 2,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'ynev.svg',
            source_type: 'infra',
          },
          num: 2,
          sample_digest: '80a7bcb4b51ad1aa876aa2144d0648bb',
        },
      ],
      commits: [
        {
          id: 'henryd84d',
          commit_time: 1573048663,
          hash: 'd84dd4babb71796ee194fa1913150d86d6aa643b',
          author: 'Henry (henry@example.com)',
          message: 'Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
    {
      groupID: '3f7c865936cc808af26d88bc1f5740a29cfce200:05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
      nDigests: 1,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'A_large_blank_world_map_with_oceans_marked_in_blue.svg',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: '02fc74ad862a6041e9ea35769a67af20',
        },
      ],
      commits: [
        {
          id: 'elisa05f6',
          commit_time: 1573151114,
          hash: '05f6a01bf9fd25be9e5fff4af5505c3945058b1d',
          author: 'Elisa (elisa@example.com)',
          message: 'Revert: Update commits to have subject information',
          cl_url: '',
        },
        {
          id: 'iris3f7c',
          commit_time: 1573151074,
          hash: '3f7c865936cc808af26d88bc1f5740a29cfce200',
          author: 'Iris (iris@example.com)',
          message: 'Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
    {
      groupID: 'e1e197186238d8d304a39db9f94258d9584a8973',
      nDigests: 1,
      nTests: 1,
      affectedTests: [
        {
          grouping: {
            name: 'gallardo.svg',
            source_type: 'infra',
          },
          num: 1,
          sample_digest: 'ed591512ff68fa088ec9aac3bb5d760d',
        },
      ],
      commits: [
        {
          id: 'irise1e1',
          commit_time: 1572460288,
          hash: 'e1e197186238d8d304a39db9f94258d9584a8973',
          author: 'Iris (iris@example.com)',
          message: 'Update commits to have subject information',
          cl_url: '',
        },
      ],
    },
  ],
};

export const trstatus: StatusResponse = {
  lastCommit: {
    id: 'bob9501',
    commit_time: 1573598625,
    hash: '9501212cd0580acfed85a90c3a16b81847fde482',
    author: 'Bob (bob@example.com)',
    message: 'Revert Update commits to have subject information',
    cl_url: '',
  },
  corpStatus: [
    {
      name: 'canvaskit',
      untriagedCount: 0,
    },
    {
      name: 'gm',
      untriagedCount: 114,
    },
    {
      name: 'svg',
      untriagedCount: 18,
    },
  ],
};
