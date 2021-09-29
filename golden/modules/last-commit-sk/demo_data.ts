import { StatusResponse } from '../rpc_types';

export const exampleStatusData: StatusResponse = {
  lastCommit: {
    id: 'foxtrota828',
    commit_time: 1598983079,
    hash: 'a8281e31afa9dddfa0764f59128c3a2360c48f49',
    author: 'Foxtrot Delta (foxtrot.delta@example.com)',
    message: 'Mark large_image_changer tests as not flaky (#65033)',
    cl_url: '',
  },
  corpStatus: [{
    name: 'flutter', untriagedCount: 0,
  }],
};
