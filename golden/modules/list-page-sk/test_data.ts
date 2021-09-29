import { ListTestsResponse } from '../rpc_types';

export const sampleByTestList: ListTestsResponse = {
  tests: [{
    name: 'this_is_a_test',
    positive_digests: 19,
    negative_digests: 24,
    untriaged_digests: 103,
    total_digests: 146,
  }, {
    name: 'this_is_another_test',
    positive_digests: 79,
    negative_digests: 48,
    untriaged_digests: 3,
    total_digests: 130,
  }],
};
