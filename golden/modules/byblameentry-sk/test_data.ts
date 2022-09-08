import { ByBlameEntry } from '../rpc_types';

export const fakeNow = Date.parse('2019-11-08T00:00:00Z');

export const entry: ByBlameEntry = {
  groupID: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  nDigests: 112,
  nTests: 7,
  affectedTests: [{
    grouping: {
      name: 'aarectmodes',
      source_type: 'infra',
    },
    num: 50,
    sample_digest: 'c6476baec94eb6a5071606575318e4df',
  }, {
    grouping: {
      name: 'aaxfermodes',
      source_type: 'infra',
    },
    num: 32,
    sample_digest: '4acfd6b3a3943cc5d75cd22e966ae6f1',
  }, {
    grouping: {
      name: 'hairmodes',
      source_type: 'infra',
    },
    num: 21,
    sample_digest: 'f9e20c63b5ce3b58d9b6a90fa3e7224c',
  }, {
    grouping: {
      name: 'imagefilters_xfermodes',
      source_type: 'infra',
    },
    num: 5,
    sample_digest: '47644613317040264fea6fa815af32e8',
  }, {
    grouping: {
      name: 'lattice2',
      source_type: 'infra',
    },
    num: 2,
    sample_digest: '16e41798ecd59b1523322a57b49cc17f',
  }, {
    grouping: {
      name: 'xfermodes',
      source_type: 'infra',
    },
    num: 1,
    sample_digest: '8fbee03f794c455c4e4842ec2736b744',
  }, {
    grouping: {
      name: 'xfermodes3',
      source_type: 'infra',
    },
    num: 1,
    sample_digest: 'fed2ff29abe371fc0ec1b2c65dfb3949',
  }],
  commits: [{
    id: 'elisabbbb',
    commit_time: 1573169814,
    hash: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    author: 'Elisa (elisa@example.com)',
    message: 'One glyph() to rule them all!!!',
    cl_url: '',
  }, {
    id: 'joeaaaa',
    commit_time: 1573149564,
    hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    author: 'Joe (joe@example.com)',
    message: 'flesh out blendmodes through Screen',
    cl_url: '',
  }],
};
