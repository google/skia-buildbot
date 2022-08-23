import { BulkTriageDeltaInfo } from '../rpc_types';

export const bulkTriageDeltaInfos: BulkTriageDeltaInfo[] = [
  {
    grouping: {
      name: 'alpha_test',
      source_type: 'animal_corpus',
    },
    digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    label_before: 'positive',
    closest_diff_label: 'positive',
    in_current_search_results_page: true,
  },
  {
    grouping: {
      name: 'alpha_test',
      source_type: 'animal_corpus',
    },
    digest: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    label_before: 'negative',
    closest_diff_label: 'negative',
    in_current_search_results_page: true,
  },
  {
    grouping: {
      name: 'alpha_test',
      source_type: 'animal_corpus',
    },
    digest: 'dddddddddddddddddddddddddddddddd',
    label_before: 'untriaged',
    closest_diff_label: 'positive',
    in_current_search_results_page: false,
  },
  {
    grouping: {
      name: 'alpha_test',
      source_type: 'plant_corpus',
    },
    digest: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    label_before: 'untriaged',
    closest_diff_label: 'negative',
    in_current_search_results_page: false,
  },
  {
    grouping: {
      name: 'beta_test',
      source_type: 'animal_corpus',
    },
    digest: 'cccccccccccccccccccccccccccccccc',
    label_before: 'untriaged',
    closest_diff_label: 'positive',
    in_current_search_results_page: true,
  },
  {
    grouping: {
      name: 'beta_test',
      source_type: 'animal_corpus',
    },
    digest: 'dddddddddddddddddddddddddddddddd',
    label_before: 'untriaged',
    closest_diff_label: 'negative',
    in_current_search_results_page: false,
  },
  {
    grouping: {
      name: 'gamma_test',
      source_type: 'animal_corpus',
    },
    digest: 'eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee',
    label_before: 'positive',
    closest_diff_label: 'none',
    in_current_search_results_page: false,
  },
];
