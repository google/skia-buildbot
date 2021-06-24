import { SearchResponse, StatusResponse, ParamSetResponse, ChangelistSummaryResponse } from '../rpc_types';

export const fakeNow = Date.parse('2020-07-20T00:00:00Z');

// Taken from https://skia-infra-gold.skia.org/json/v1/trstatus on 2020-07-15.
export const statusResponse: StatusResponse = {
  "lastCommit": {
    "id": "Dan3e53",
    "commit_time": 1594817699,
    "hash": "3e53cd963f319a3e3e293bd091e83149eab703f6",
    "author": "Dan (dan@example.com)",
    "message": "[autoroll] Fixes",
    "cl_url": ""
  },
  "corpStatus": [{
    "name": "infra",
    "untriagedCount": 18
  },
  {
    "name": "my-corpus",
    "untriagedCount": 5
  }]
};

// Taken from https://skia-infra-gold.skia.org/json/v1/paramset on 2020-07-15.
export const paramSetResponse: ParamSetResponse = {
  "ext": ["png"],
  "name": [
    "am_email-chooser-sk",
    "am_email-chooser-sk_non-owner-selected",
    "am_email-chooser-sk_owner-selected",
    "ct_suggest-input-sk",
    "ct_suggest-input-sk_suggestions",
    "ct_task-queue-sk",
    "ct_task-queue-sk_delete",
    "ct_task-queue-sk_task-details",
    "gold_blamelist-panel-sk",
    "gold_blamelist-panel-sk_cl-commit",
    "gold_blamelist-panel-sk_many-commits",
    "gold_blamelist-panel-sk_some-commits",
    "gold_bulk-triage-sk_changelist",
    "gold_bulk-triage-sk_closest",
    "gold_bulk-triage-sk_negative",
    "gold_bulk-triage-sk_positive",
    "gold_bulk-triage-sk_positive-button-focused",
    "gold_bulk-triage-sk_triage-all",
    "gold_bulk-triage-sk_untriaged",
    "gold_byblame-page-sk",
    "gold_byblame-page-sk_limits-blamelist-commits",
    "gold_byblameentry-sk",
    "gold_changelist-controls-sk",
    "gold_changelists-page-sk",
    "gold_changelists-page-sk_show-all",
    "gold_cluster-digests-sk",
    "gold_cluster-digests-sk_one-positive-selected",
    "gold_cluster-digests-sk_one-untriaged-selected",
    "gold_cluster-digests-sk_three-digests-selected",
    "gold_cluster-digests-sk_two-digests-selected",
    "gold_cluster-page-sk",
    "gold_cluster-page-sk_one-digest-selected",
    "gold_cluster-page-sk_three-digests-selected",
    "gold_cluster-page-sk_two-digests-selected",
    "gold_corpus-selector-sk",
    "gold_corpus-selector-sk_custom-fn",
    "gold_corpus-selector-sk_long-strings",
    "gold_details-page-sk",
    "gold_details-page-sk_backend-error",
    "gold_details-page-sk_not-in-index",
    "gold_diff-page-sk",
    "gold_digest-details-sk",
    "gold_digest-details-sk_changelist-id",
    "gold_digest-details-sk_negative-only",
    "gold_digest-details-sk_no-params",
    "gold_digest-details-sk_no-refs",
    "gold_digest-details-sk_no-traces",
    "gold_digest-details-sk_right-overridden",
    "gold_dots-legend-sk",
    "gold_dots-legend-sk_too-many-digests",
    "gold_dots-sk",
    "gold_dots-sk_highlighted",
    "gold_edit-ignore-rule-sk",
    "gold_edit-ignore-rule-sk_missing-custom-value",
    "gold_edit-ignore-rule-sk_missing-data",
    "gold_edit-ignore-rule-sk_with-data",
    "gold_filter-dialog-sk",
    "gold_filter-dialog-sk_query-dialog-open",
    "gold_filter-dialog-sk_query-editor-open",
    "gold_gold-scaffold-sk",
    "gold_ignores-page-sk",
    "gold_ignores-page-sk_all-traces",
    "gold_ignores-page-sk_create-modal",
    "gold_ignores-page-sk_delete-dialog",
    "gold_ignores-page-sk_update-modal",
    "gold_image-compare-sk",
    "gold_image-compare-sk_no-right",
    "gold_image-compare-sk_zoom-dialog",
    "gold_list-page-sk",
    "gold_list-page-sk_query-dialog",
    "gold_multi-zoom-sk",
    "gold_multi-zoom-sk_base64-small",
    "gold_multi-zoom-sk_mismatch",
    "gold_multi-zoom-sk_nth-pixel",
    "gold_multi-zoom-sk_zoomed-grid",
    "gold_query-dialog-sk_key-and-value-selected",
    "gold_query-dialog-sk_key-selected",
    "gold_query-dialog-sk_multiple-values-selected",
    "gold_query-dialog-sk_no-selection",
    "gold_query-dialog-sk_nonempty-initial-selection",
    "gold_search-controls-sk",
    "gold_search-controls-sk_empty",
    "gold_search-controls-sk_left-hand-trace-filter-editor",
    "gold_search-controls-sk_more-filters",
    "gold_search-controls-sk_right-hand-trace-filter-editor",
    "gold_trace-filter-sk",
    "gold_trace-filter-sk_nonempty",
    "gold_trace-filter-sk_nonempty_query-dialog-open",
    "gold_trace-filter-sk_query-dialog-open",
    "gold_triage-history-sk",
    "gold_triage-sk_empty",
    "gold_triage-sk_negative",
    "gold_triage-sk_positive",
    "gold_triage-sk_positive-button-focused",
    "gold_triage-sk_untriaged",
    "gold_triagelog-page-sk",
    "infra-sk_autogrow-textarea-sk",
    "infra-sk_autogrow-textarea-sk_filled",
    "infra-sk_autogrow-textarea-sk_grow",
    "infra-sk_autogrow-textarea-sk_shrink",
    "infra-sk_expandable-textarea-sk_closed",
    "infra-sk_expandable-textarea-sk_open",
    "infra-sk_paramset-sk_many-paramsets_no-titles",
    "infra-sk_paramset-sk_many-paramsets_with-titles",
    "infra-sk_paramset-sk_one-paramset_no-titles",
    "infra-sk_paramset-sk_one-paramset_with-titles",
    "infra-sk_query-sk",
    "infra-sk_sort-sk",
    "perf_alert-config-sk",
    "perf_alerts-page-sk",
    "perf_algo-select-sk",
    "perf_calendar-input-sk",
    "perf_calendar-sk",
    "perf_cluster-lastn-page-sk",
    "perf_cluster-page-sk",
    "perf_cluster-summary2-sk",
    "perf_commit-detail-panel-sk",
    "perf_commit-detail-picker-sk",
    "perf_commit-detail-sk",
    "perf_day-range-sk",
    "perf_day-range-sk_begin-selector",
    "perf_day-range-sk_end-selector",
    "perf_domain-picker-sk",
    "perf_json-source-sk",
    "perf_perf-scaffold-sk",
    "perf_plot-simple-sk",
    "perf_query-chooser-sk",
    "perf_query-count-sk",
    "perf_query-sk",
    "perf_sort-sk",
    "perf_triage-status-sk",
    "perf_triage2-sk",
    "perf_tricon2-sk",
    "perf_word-cloud-sk"
  ],
  "source_type": ["infra"]
};

// Taken from https://skia-infra-gold.skia.org/json/changelist/gerrit/31719 on 2020-09-15.
export const changeListSummaryResponse: ChangelistSummaryResponse = {
  "cl": {
    "system": "gerrit",
    "id": "123456",
    "owner": "alice@example.com",
    "status": "Open",
    "subject": "Make some interesting changes here and there",
    "updated": "2020-09-15T21:04:19.662357Z",
    "url": "https://skia-review.googlesource.com/123456"
  },
  "patch_sets": [{
    "id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "order": 1,
    "try_jobs": [{
      "id": "11111111111111111111",
      "name": "Infra-PerCommit-Puppeteer",
      "updated": "2020-09-15T11:11:11.111111Z",
      "system": "buildbucket",
      "url": "https://cr-buildbucket.appspot.com/build/11111111111111111111"
    }]
  }, {
    "id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "order": 2,
    "try_jobs": [{
      "id": "22222222222222222222",
      "name": "Infra-PerCommit-Puppeteer",
      "updated": "2020-09-15T22:22:22.222222Z",
      "system": "buildbucket",
      "url": "https://cr-buildbucket.appspot.com/build/22222222222222222222"
    }]
  }],
  "num_total_patch_sets": 2
};

// Taken from https://skia-infra-gold.skia.org/json/v1/search on 2020-07-15. Trimmed and anonymized.
export const searchResponse: SearchResponse = {
  "digests": [{
      "digest": "fbd3de3fff6b852ae0bb6751b9763d27",
      "test": "gold_search-controls-sk_right-hand-trace-filter-editor",
      "status": "positive",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
          "source_type": ["infra"]
      },
      "traces": {
          "traces": [{
              "label": ",name=gold_search-controls-sk_right-hand-trace-filter-editor,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_right-hand-trace-filter-editor",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "fbd3de3fff6b852ae0bb6751b9763d27",
              "status": "positive"
          }, {
              "digest": "0b6e67b8c8123a3fce7f3a98ef0ea71d",
              "status": "positive"
          }, {
              "digest": "d20f37006e436fe17f50ecf49ff2bdb5",
              "status": "positive"
          }, {
              "digest": "5d8c80eda80e015d633a4125ab0232dc",
              "status": "positive"
          }, {
              "digest": "88aa1cdc50433c0ec4404485eeb63b69",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2401,
              "pixelDiffPercent": 0.25010416,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "combinedMetric": 0.0921628,
              "digest": "5d8c80eda80e015d633a4125ab0232dc",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
                  "source_type": ["infra"]
              }
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "2fa58aa430e9c815755624ca6cca4a72",
      "test": "perf_alert-config-sk",
      "status": "negative",
      "triage_history": [{
          "user": "bob@example.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alert-config-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "traces": [{
              "label": ",name=perf_alert-config-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 8, 4, 4, 4, 4, 4, 4, 4, 8, 8, 7, 7, 7, 7, 7, 8, 0, 0, 8, 0, 0, 0, 0, 0, 8, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 6, 6, 6, 6, 6, 6, 3, 1, 1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_alert-config-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "2fa58aa430e9c815755624ca6cca4a72",
              "status": "negative"
          }, {
              "digest": "03fc26ba0daa6b31dc95a1cf38ae8085",
              "status": "untriaged"
          }, {
              "digest": "1691a88362b8e8aa8fa04d67abdf389d",
              "status": "untriaged"
          }, {
              "digest": "773778cb89f8a13870a0a52f1164a813",
              "status": "untriaged"
          }, {
              "digest": "d65787215992b0bfef6dc25fe69edeb6",
              "status": "positive"
          }, {
              "digest": "7f3abcb9af187bf125f4a869250a8ef4",
              "status": "positive"
          }, {
              "digest": "819d37f3491654038abbbfe1f94d56ac",
              "status": "untriaged"
          }, {
              "digest": "f147acaa7691235e659873a2eef3b5b9",
              "status": "untriaged"
          }, {
              "digest": "1be6f30e715b5db5a7b5363872514c91",
              "status": "untriaged"
          }],
          "total_digests": 12
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 3880,
              "pixelDiffPercent": 0.110857144,
              "maxRGBADiffs": [33, 33, 33, 0],
              "dimDiffer": false,
              "combinedMetric": 0.11146385,
              "digest": "ed4a8cf9ea9fbb57bf1f302537e07572",
              "status": "untriaged",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alert-config-sk"],
                  "source_type": ["infra"]
              }
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ed4a8cf9ea9fbb57bf1f302537e07572",
      "test": "perf_alert-config-sk",
      "status": "untriaged",
      "triage_history": [{
          "user": "bob@example.com",
          "ts": "2020-07-09T15:06:59.585035Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alert-config-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "traces": [{
              "label": ",name=perf_alert-config-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 8, 5, 5, 5, 5, 5, 5, 5, 8, 8, 8, 8, 8, 8, 8, 0, 4, 4, 8, 4, 4, 4, 4, 4, 8, 4, 8, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 7, 7, 7, 7, 7, 7, 7, 3, 1, 1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_alert-config-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "ed4a8cf9ea9fbb57bf1f302537e07572",
              "status": "untriaged"
          }, {
              "digest": "03fc26ba0daa6b31dc95a1cf38ae8085",
              "status": "untriaged"
          }, {
              "digest": "1691a88362b8e8aa8fa04d67abdf389d",
              "status": "untriaged"
          }, {
              "digest": "773778cb89f8a13870a0a52f1164a813",
              "status": "untriaged"
          }, {
              "digest": "2fa58aa430e9c815755624ca6cca4a72",
              "status": "negative"
          }, {
              "digest": "d65787215992b0bfef6dc25fe69edeb6",
              "status": "positive"
          }, {
              "digest": "7f3abcb9af187bf125f4a869250a8ef4",
              "status": "positive"
          }, {
              "digest": "819d37f3491654038abbbfe1f94d56ac",
              "status": "untriaged"
          }, {
              "digest": "f147acaa7691235e659873a2eef3b5b9",
              "status": "untriaged"
          }],
          "total_digests": 12
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 3880,
              "pixelDiffPercent": 0.110857144,
              "maxRGBADiffs": [33, 33, 33, 0],
              "dimDiffer": false,
              "combinedMetric": 0.11146385,
              "digest": "2fa58aa430e9c815755624ca6cca4a72",
              "status": "negative",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alert-config-sk"],
                  "source_type": ["infra"]
              }
          }
      },
      "closestRef": "pos"
  }],
  "offset": 0,
  "size": 85,
  "commits": [{
      "id": "Alice915a",
      "commit_time": 1592422850,
      "hash": "915a4938104e09e50b0f148220436ee9dfe3148e",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Port list-page-sk to lit-html",
      "cl_url": ""
  }, {
      "id": "Alicef52c",
      "commit_time": 1592422850,
      "hash": "f52c8f23cc673f13c5880da3c109ea4a5aed8cb3",
      "author": "Alice (alice@example.com)",
      "message": "[gold] use new by-list page",
      "cl_url": ""
  }, {
      "id": "Alicef067",
      "commit_time": 1592422850,
      "hash": "f0678718512e1e4ad8f9ae842964fa2568a7e315",
      "author": "Alice (alice@example.com)",
      "message": "[gold] delete old list page",
      "cl_url": ""
  }, {
      "id": "Alicefc00",
      "commit_time": 1592423578,
      "hash": "fc002ae64a91fff06f26480a14ea93d34e6393e0",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Fix links on triagelog page",
      "cl_url": ""
  }, {
      "id": "Alice7c98",
      "commit_time": 1592425413,
      "hash": "7c98a621f9ce0ef2a5bece266419c56e8a3d7970",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Tweak trace dots color palette",
      "cl_url": ""
  }, {
      "id": "Alice8926",
      "commit_time": 1592425413,
      "hash": "892675778a25b7e15468028c10834637f84d8b7d",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Re-assign digest colors, accounting for most used.",
      "cl_url": ""
  }, {
      "id": "Alice9684",
      "commit_time": 1592426938,
      "hash": "9684e478f6fb4b41de1631ffda1cbfa16c5da523",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Link to source code with full blamelist.",
      "cl_url": ""
  }, {
      "id": "Alicea73f",
      "commit_time": 1592426948,
      "hash": "a73fdcf1a4df261928edf78df87dfcad5653b96e",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add notes to debug firestore usage",
      "cl_url": ""
  }, {
      "id": "Bob0836",
      "commit_time": 1592445320,
      "hash": "08368432736162e44e9e1f96140991faf1c69024",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix demo pages.",
      "cl_url": ""
  }, {
      "id": "Bobbff7",
      "commit_time": 1592453179,
      "hash": "bff785024d44423e41697bbe5e63fe67a4b24565",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Convert day-range-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Carolf25e",
      "commit_time": 1592479586,
      "hash": "f25e7a6613dc31255cc99187004a2a2ce9bb7d0c",
      "author": "Carol (carol@example.com)",
      "message": "Update child branch for shaderc",
      "cl_url": ""
  }, {
      "id": "Dana80a",
      "commit_time": 1592481416,
      "hash": "a80ad7d70f66f5cffb195bbc5249b76b5a1dce94",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] Obtain CIPD package details and bugs from tags",
      "cl_url": ""
  }, {
      "id": "Alice1838",
      "commit_time": 1592494869,
      "hash": "1838cb950ea16f1396c1d5dcbdf846c5ea1529db",
      "author": "Alice (alice@example.com)",
      "message": "[infra] Add file and function name to firestore metrics.",
      "cl_url": ""
  }, {
      "id": "Alice88ed",
      "commit_time": 1592494869,
      "hash": "88ed2a7fbdd8ae9c521770e30b81b007966fb507",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add more metrics to search and indexer.",
      "cl_url": ""
  }, {
      "id": "Bob94f2",
      "commit_time": 1592501876,
      "hash": "94f2cbe3721ead8f39deea1d493aad078e72287e",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port query-chooser-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bob7d16",
      "commit_time": 1592503126,
      "hash": "7d165892fd9fb7565b27a39a2c6446f4a2072886",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port commit-detail-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Erin1694",
      "commit_time": 1592505416,
      "hash": "16941f1a2bc61b3f276c404d93b4eb88a8a78631",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] Makefile: Update puppeteer_tests target.",
      "cl_url": ""
  }, {
      "id": "Alice4e0b",
      "commit_time": 1592505943,
      "hash": "4e0bd800a0d6b2c10cd33230efb0f98940d19b39",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Update metrics and frontend",
      "cl_url": ""
  }, {
      "id": "Aliced8c3",
      "commit_time": 1592506589,
      "hash": "d8c3ad848d26489f8129ab031a1e516682b18cd1",
      "author": "Alice (alice@example.com)",
      "message": "[gold] re-enable comments for chrome",
      "cl_url": ""
  }, {
      "id": "Franka55a",
      "commit_time": 1592507867,
      "hash": "a55a0c2b25192900d658095d180b0aca63148a3d",
      "author": "Frank (frank@example.com)",
      "message": "Add GetReference and CreateReference to go/github",
      "cl_url": ""
  }, {
      "id": "Erin2c40",
      "commit_time": 1592509966,
      "hash": "2c4080564b80d0727cb4fc2b2fbf01d267e367d5",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] Fix Puppeteer tests.",
      "cl_url": ""
  }, {
      "id": "Erincfe9",
      "commit_time": 1592509986,
      "hash": "cfe951e940d914565acdaefc8bf12bdbcf1c37ab",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] Include Puppeteer tests in Infra-PerCommit-Puppeteer task.",
      "cl_url": ""
  }, {
      "id": "Erin0337",
      "commit_time": 1592510888,
      "hash": "033768078c1dcf7f898e72bc356faeffb5f26035",
      "author": "Erin (erin@example.com)",
      "message": "[gold] search-controls-sk.html: Remove obsolete call to the Polymer-based query-dialog-sk's close() method.",
      "cl_url": ""
  }, {
      "id": "Gracefcf6",
      "commit_time": 1592536294,
      "hash": "fcf6dfc729de968ce35f051a764f7763c8ee3b24",
      "author": "Grace (grace@example.com)",
      "message": "Make eventPromise listen to document, so error-sk is caught.",
      "cl_url": ""
  }, {
      "id": "Alicea082",
      "commit_time": 1592570749,
      "hash": "a08250a3e8c23d020331bfa00bcd9d64800c3a8b",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add (somewhat) helpful help page",
      "cl_url": ""
  }, {
      "id": "Bob3060",
      "commit_time": 1592574950,
      "hash": "3060cfae7069e850b7875f46a21d8f5a48a5f396",
      "author": "Bob (bob@example.com)",
      "message": "[infra-sk] Add gold tests for query-sk.",
      "cl_url": ""
  }, {
      "id": "Alicedc56",
      "commit_time": 1592579790,
      "hash": "dc56391f171b283c4898a7076f7530f2c0e3ae54",
      "author": "Alice (alice@example.com)",
      "message": "Delete dead metrics code",
      "cl_url": ""
  }, {
      "id": "Aliceb18a",
      "commit_time": 1592580020,
      "hash": "b18a31da1f061769ee8737491a71d307ae084466",
      "author": "Alice (alice@example.com)",
      "message": "Fix egde typo (again)",
      "cl_url": ""
  }, {
      "id": "Alice5f61",
      "commit_time": 1592589807,
      "hash": "5f614e46f5adb97185d25c97dc176e705079a751",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Don't error on CL not found",
      "cl_url": ""
  }, {
      "id": "Bobf750",
      "commit_time": 1592616559,
      "hash": "f750590147996aeb901535dff9ff19e3f2285574",
      "author": "Bob (bob@example.com)",
      "message": "sort-sk to typescript.",
      "cl_url": ""
  }, {
      "id": "Bobcab3",
      "commit_time": 1592661916,
      "hash": "cab39d1442120ce5f5f1ae60c39685073158ffc4",
      "author": "Bob (bob@example.com)",
      "message": "Fix appname for some infra-sk tests.",
      "cl_url": ""
  }, {
      "id": "Bob23b7",
      "commit_time": 1592665760,
      "hash": "23b7fda631d54cdddad762a68671df6fcf6efd76",
      "author": "Bob (bob@example.com)",
      "message": "algo-select-ts",
      "cl_url": ""
  }, {
      "id": "Liam9d47",
      "commit_time": 1592717000,
      "hash": "9d47158ed15a3f70f1650f63eba61d665bc4a494",
      "author": "Liam (liam@example.com)",
      "message": "Update CIPD Packages",
      "cl_url": ""
  }, {
      "id": "Frank6cf7",
      "commit_time": 1592743920,
      "hash": "6cf7e3a36a9fb9540f36f32895fd3bf6a18c77d7",
      "author": "Frank (frank@example.com)",
      "message": "[Autorollers] Add supportsManualRoll to Skia-\u003eFlutter",
      "cl_url": ""
  }, {
      "id": "Graceec84",
      "commit_time": 1592844302,
      "hash": "ec8409605ca8f5cf48ed463bd5c270637e0847f3",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add value-changed event to suggest-input-sk",
      "cl_url": ""
  }, {
      "id": "Grace3632",
      "commit_time": 1592846032,
      "hash": "3632b9ef7dd885f962249c959983a32aa3f4ef76",
      "author": "Grace (grace@example.com)",
      "message": "[infra-sk] Manually recompute textarea size on expand in expandable-textarea-sk. Necessary if value is set prior to making the element visible.",
      "cl_url": ""
  }, {
      "id": "Dan1449",
      "commit_time": 1592848997,
      "hash": "144993a82a595aeffd73ec461e021bbd7c8a5ab3",
      "author": "Dan (dan@example.com)",
      "message": "[task driver] Fix nil-dereference in display",
      "cl_url": ""
  }, {
      "id": "Erin8a05",
      "commit_time": 1592849157,
      "hash": "8a05f82240eeeef2fb4abc25a1ed7eb01efcb031",
      "author": "Erin (erin@example.com)",
      "message": "[gold] Port filter-dialog-sk to lit-html.",
      "cl_url": ""
  }, {
      "id": "Alice7d3c",
      "commit_time": 1592849216,
      "hash": "7d3c4130eca10b626b24b52d9bec79bd2a3976f6",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add chrome service account to frontend",
      "cl_url": ""
  }, {
      "id": "Alice56be",
      "commit_time": 1592849987,
      "hash": "56bef8bee90d5832fa5091896a8ba4f9e4444558",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Fix bare html",
      "cl_url": ""
  }, {
      "id": "Erin8e9a",
      "commit_time": 1592850407,
      "hash": "8e9a9b9a5455ab6ee1ba8846acdfccd4507b3b36",
      "author": "Erin (erin@example.com)",
      "message": "[gold] Use the lit-html version of filter-dialog-sk.",
      "cl_url": ""
  }, {
      "id": "Gracea5d1",
      "commit_time": 1592854507,
      "hash": "a5d185a27c433727d267196617ea85d671c0ab1c",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add chromium-perf-sk element.",
      "cl_url": ""
  }, {
      "id": "Erin0959",
      "commit_time": 1592904723,
      "hash": "095957759778ccee8b00fe2a1cf27812390b04df",
      "author": "Erin (erin@example.com)",
      "message": "[gold] search-controls-sk: Initial skeleton code.",
      "cl_url": ""
  }, {
      "id": "Dan26c6",
      "commit_time": 1592914773,
      "hash": "26c69790a4f6114bc22578017a235444e0dbe9b2",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] No error for failure to retrieve sheriff",
      "cl_url": ""
  }, {
      "id": "Bob4813",
      "commit_time": 1592923983,
      "hash": "48137d2ce6d2d0f3c6b8886d5a6abe84a836031d",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Add new bucket for Flutter instance.",
      "cl_url": ""
  }, {
      "id": "Bob7ecf",
      "commit_time": 1592931683,
      "hash": "7ecf290a1fc8e4851d0caf7cd973e021b84929e1",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Remove skiaperf executable.",
      "cl_url": ""
  }, {
      "id": "Frank9f9d",
      "commit_time": 1592938014,
      "hash": "9f9d4316defdfdd549cd26e2e8596d885b7b42e9",
      "author": "Frank (frank@example.com)",
      "message": "[Autoroller] Cleanup github fork branches older than a week",
      "cl_url": ""
  }, {
      "id": "Bobb195",
      "commit_time": 1592940083,
      "hash": "b195e89bbf497c505f3940a923ca9ea7cf8b2745",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Remove unused flag.",
      "cl_url": ""
  }, {
      "id": "Bob236e",
      "commit_time": 1592941623,
      "hash": "236e1d7dae45212d69e453fcbb27191dfc082d42",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix cluster sub-command.",
      "cl_url": ""
  }, {
      "id": "Grace02bd",
      "commit_time": 1592941743,
      "hash": "02bd88e3cf6c7280448b0494aaa30328332c4574",
      "author": "Grace (grace@example.com)",
      "message": "[demos] Add public service account script",
      "cl_url": ""
  }, {
      "id": "Grace5825",
      "commit_time": 1592942573,
      "hash": "5825563c657ba4333a0b142cc816f17e7b7bb797",
      "author": "Grace (grace@example.com)",
      "message": "[demos] Update skfe for demos",
      "cl_url": ""
  }, {
      "id": "Alice9028",
      "commit_time": 1593002489,
      "hash": "9028592373a3ee5a295cc2f1208309a4d62e9f8b",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Update images",
      "cl_url": ""
  }, {
      "id": "Gracecab9",
      "commit_time": 1593010049,
      "hash": "cab905effaf1c8a4e95b09f2ce2141cfe2875d88",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Limit link color CSS to patch-sk",
      "cl_url": ""
  }, {
      "id": "Gracec9ac",
      "commit_time": 1593010679,
      "hash": "c9ac1dc9e239db21ad6d42679b1fe51cbf9b064f",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Fix ct-scaffold chromium_perf link.",
      "cl_url": ""
  }, {
      "id": "Grace9e1c",
      "commit_time": 1593026189,
      "hash": "9e1cf3de4df77b2b9a797f3462374c029ed8bf23",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add chromium_perf page.",
      "cl_url": ""
  }, {
      "id": "Bobbe64",
      "commit_time": 1593094640,
      "hash": "be64fcd3950350f08966b178951d8a159d2b93b2",
      "author": "Bob (bob@example.com)",
      "message": "[query-sk] Fix regex queries.",
      "cl_url": ""
  }, {
      "id": "Bobc23c",
      "commit_time": 1593181061,
      "hash": "c23c2aa69fc1d19e2059ba0c60bdd8108a52281f",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Convert perf-scaffold-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Alice3262",
      "commit_time": 1593181222,
      "hash": "32627f70fbc59fc2a9293e6a85bb8bf92de761c7",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Make puppeteer tests much faster and use less RAM.",
      "cl_url": ""
  }, {
      "id": "Grace4d37",
      "commit_time": 1593193671,
      "hash": "4d37bf5d78c7a20cd6f1e9e341640af010222c06",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Fix missing handler for Runs History in chromium-perf-sk",
      "cl_url": ""
  }, {
      "id": "Erin0506",
      "commit_time": 1593195301,
      "hash": "0506beb1a6f910b9b30f9eacbc20b18e18d763c7",
      "author": "Erin (erin@example.com)",
      "message": "[gold] Extract trace-filter-sk component out of filter-dialog-sk.",
      "cl_url": ""
  }, {
      "id": "Erind778",
      "commit_time": 1593197161,
      "hash": "d7783ba31f7b41330e3d972c0195addd8fc890a0",
      "author": "Erin (erin@example.com)",
      "message": "[gold] corpus-selector-sk: Port to TypeScript and simplify.",
      "cl_url": ""
  }, {
      "id": "Erinb9f6",
      "commit_time": 1593200691,
      "hash": "b9f63db4dbe86f23c8eeac97f220475f2f246391",
      "author": "Erin (erin@example.com)",
      "message": "[gold] search-controls-sk: Implement lit-html component.",
      "cl_url": ""
  }, {
      "id": "Bobad61",
      "commit_time": 1593200701,
      "hash": "ad618eb5a4bbc52c22925b1252f469e07f054cac",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Convert commit-detail-panel-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bob1be3",
      "commit_time": 1593204121,
      "hash": "1be3fbb1e03af485133d586369f1505f2d1fecf9",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port commit-detail-picker-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bobc630",
      "commit_time": 1593204711,
      "hash": "c630087bf6b70e30a1dd291272e013e278749921",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix randomness in plot-simple-sk Gold images.",
      "cl_url": ""
  }, {
      "id": "Bobb7b8",
      "commit_time": 1593206841,
      "hash": "b7b816d7e1694ff07a8a99f9c229510940f39845",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Experiment with tslint.",
      "cl_url": ""
  }, {
      "id": "Erin6028",
      "commit_time": 1593228421,
      "hash": "60281e15926171777cc997ea02ab2cdd600edd5e",
      "author": "Erin (erin@example.com)",
      "message": "[gold] search-controls-sk: Use the new lit-html component everywhere.",
      "cl_url": ""
  }, {
      "id": "Liamccb3",
      "commit_time": 1593321783,
      "hash": "ccb305d05cd36cf81dfa8712daef092848a7c032",
      "author": "Liam (liam@example.com)",
      "message": "Update CIPD Packages",
      "cl_url": ""
  }, {
      "id": "Grace026a",
      "commit_time": 1593442235,
      "hash": "026a27eb2582113e3377537bb08a8ac24c43387b",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add chromium-analysis-sk element.",
      "cl_url": ""
  }, {
      "id": "Alice2be3",
      "commit_time": 1593448162,
      "hash": "2be37a93d661b72ac1ad76779095b9e867c542a0",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Make goldpushk relative from golden subfolder",
      "cl_url": ""
  }, {
      "id": "Aliceae98",
      "commit_time": 1593448162,
      "hash": "ae98cd6c5590603a3e08b28d890c82772efdb546",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Port diffserver to using JSON config",
      "cl_url": ""
  }, {
      "id": "Alice67ad",
      "commit_time": 1593448162,
      "hash": "67ad34a54b6135241853eaeb84bc5a970db94814",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Refactor ingestion configs",
      "cl_url": ""
  }, {
      "id": "Alice0657",
      "commit_time": 1593448162,
      "hash": "065737b1c16da25a970f5928e95537ab2bb728d1",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Port ingestion-bt to use JSON instead of flags+JSON.",
      "cl_url": ""
  }, {
      "id": "Alicedba6",
      "commit_time": 1593448162,
      "hash": "dba6b285e59ed0f23c2fd1661246097fd50a1f5c",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Port final service to JSON5 config. Delete old configs.",
      "cl_url": ""
  }, {
      "id": "Bob5b43",
      "commit_time": 1593450044,
      "hash": "5b4399509651557033f47216c82f6bfb45f8d1e9",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Standup perf flutter-flutter instance.",
      "cl_url": ""
  }, {
      "id": "Alice5f26",
      "commit_time": 1593451773,
      "hash": "5f2682aa9683a86c5e0ce3b61862e02c3e46b8af",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Update to clean images using JSON config",
      "cl_url": ""
  }, {
      "id": "Bob08f1",
      "commit_time": 1593452815,
      "hash": "08f1164afdb2e4aa803a4610e6e8398a83587abf",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix for plot-simple-sk.",
      "cl_url": ""
  }, {
      "id": "Bob6809",
      "commit_time": 1593452985,
      "hash": "6809c193328f1f6cc23920c35e4aefc717c971a1",
      "author": "Bob (bob@example.com)",
      "message": "[perf] triage-status fixes.",
      "cl_url": ""
  }, {
      "id": "Bobf281",
      "commit_time": 1593452995,
      "hash": "f281e74bfcead8385664b08e722dfd267ccd31b9",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Move flutter-perf.skia.org to flutter-engine-perf.skia.org",
      "cl_url": ""
  }, {
      "id": "Alicefb07",
      "commit_time": 1593453835,
      "hash": "fb0713f028085c39e77e2591942ed97e463f5c0e",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Make indexer use timestamp on tryjob results.",
      "cl_url": ""
  }, {
      "id": "Alice4c0f",
      "commit_time": 1593453835,
      "hash": "4c0fef1593677728bd2394ee148ab58c38bd445a",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Use time in tryjob results",
      "cl_url": ""
  }, {
      "id": "Alice6448",
      "commit_time": 1593456285,
      "hash": "6448e5e770bcc94c28ee6ea0af2839347af8b6cd",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Fix config and push new ingestion",
      "cl_url": ""
  }, {
      "id": "Alice9fa0",
      "commit_time": 1593529394,
      "hash": "9fa0af1e2e8b126c7f9cf57472c11c7444d87b9d",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add anti-affinity to improve uptime",
      "cl_url": ""
  }, {
      "id": "Bob6d11",
      "commit_time": 1593530721,
      "hash": "6d1199459660b810dda7fb9a1748281989c54e0f",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Redirect to flutter at the new address.",
      "cl_url": ""
  }, {
      "id": "Bob7007",
      "commit_time": 1593533965,
      "hash": "7007683e3627a013d42e04a342800609ba164f45",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix selectable on commit-detail-panel-sk instances.",
      "cl_url": ""
  }, {
      "id": "Bob2848",
      "commit_time": 1593539085,
      "hash": "28484fe00d2b3ab501153966fa872ff3d9d154b8",
      "author": "Bob (bob@example.com)",
      "message": "[perf] calendar-sk",
      "cl_url": ""
  }, {
      "id": "Dan5bf4",
      "commit_time": 1593541135,
      "hash": "5bf4085d29f17e40378ce0b39cd8883c9d94ba17",
      "author": "Dan (dan@example.com)",
      "message": "[webtools] Use \"npm ci\" instead of \"npm install\"",
      "cl_url": ""
  }, {
      "id": "Liamd944",
      "commit_time": 1593580945,
      "hash": "d944fa9523637794c63a853d40aff04a3cc64b0a",
      "author": "Liam (liam@example.com)",
      "message": "Update Go Deps",
      "cl_url": ""
  }, {
      "id": "Bobcda3",
      "commit_time": 1593617328,
      "hash": "cda3473da077e7372bc350e002ea96abf9525706",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Add perf-tool backup command.",
      "cl_url": ""
  }, {
      "id": "Henry27fa",
      "commit_time": 1593621018,
      "hash": "27fa3d05a90511fdba7817b229faf167269694cd",
      "author": "Henry (henry@example.com)",
      "message": "Update PGO Autorollers with Beta and Contact Info",
      "cl_url": ""
  }, {
      "id": "Frank7de5",
      "commit_time": 1593624186,
      "hash": "7de50cb43bff0f5d932d7eccba9bb01434d6c53f",
      "author": "Frank (frank@example.com)",
      "message": "[CT] Fix bug where run_on_gce=false for Windows",
      "cl_url": ""
  }, {
      "id": "Gracef984",
      "commit_time": 1593625457,
      "hash": "f9847d7a3351d4a85b82bff949de8e9e8cb830c3",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add chromium_analysis page",
      "cl_url": ""
  }, {
      "id": "Franka910",
      "commit_time": 1593631566,
      "hash": "a910315e976e556253015f23c6f4e527a049cbf5",
      "author": "Frank (frank@example.com)",
      "message": "[debugger_assets] Fix broken build target",
      "cl_url": ""
  }, {
      "id": "Grace3f0e",
      "commit_time": 1593632486,
      "hash": "3f0ed381eedc05be8fd8468385791d42ac744cb9",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Re-render ct-scaffold once we have queue length.",
      "cl_url": ""
  }, {
      "id": "Alicea4f0",
      "commit_time": 1593632600,
      "hash": "a4f02654ea2ce1d692c60fa0069f80fb8fac5449",
      "author": "Alice (alice@example.com)",
      "message": "[sort-sk] Support use in table and expose sort()",
      "cl_url": ""
  }, {
      "id": "Alice18cb",
      "commit_time": 1593632600,
      "hash": "18cbfe7552632f5927dc6b4ba065ead9815fb8be",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Use sort-sk on list-page-sk",
      "cl_url": ""
  }, {
      "id": "Alice0e2b",
      "commit_time": 1593632600,
      "hash": "0e2b9fff86153d611df6af451d67f81f5fdb1750",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add links to by-list for subset of digests",
      "cl_url": ""
  }, {
      "id": "Bob4cbf",
      "commit_time": 1593634876,
      "hash": "4cbfff6d6e4d23a552981218a64fd2fa0c657eef",
      "author": "Bob (bob@example.com)",
      "message": "[perf] calendar-input-sk.",
      "cl_url": ""
  }, {
      "id": "Bobcfad",
      "commit_time": 1593639986,
      "hash": "cfad866369daddc5b9f208ed8ff53f24f1155c91",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Move day-range-sk from input type=date to calendar-input-sk.",
      "cl_url": ""
  }, {
      "id": "Frankf650",
      "commit_time": 1593646037,
      "hash": "f65049e8d1995b59bfebe2c39f951cc945ffff4f",
      "author": "Frank (frank@example.com)",
      "message": "[CT] Remove SK_WHITELIST_SERIALIZED_TYPEFACES from CT",
      "cl_url": ""
  }, {
      "id": "Bob9ec9",
      "commit_time": 1593709667,
      "hash": "9ec95c07091380e3991e4b9d9f92f4c163a6e4df",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port domain-picker-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bobfa72",
      "commit_time": 1593832132,
      "hash": "fa7211d2ad9becb0791d424f8429ce7144635a18",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port clsuter-summary2-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Liam5b85",
      "commit_time": 1593926413,
      "hash": "5b8591b50b2475f259fc2c73692a8ce195e2cfdf",
      "author": "Liam (liam@example.com)",
      "message": "Update CIPD Packages",
      "cl_url": ""
  }, {
      "id": "Bobe317",
      "commit_time": 1593959214,
      "hash": "e317f954b75893979a875f50ba32e2e4f099b367",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port alert-config-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Liamd75a",
      "commit_time": 1594012779,
      "hash": "d75a88d1bb625bf39335d97cbd464b9e29f5624b",
      "author": "Liam (liam@example.com)",
      "message": "Update Go Deps",
      "cl_url": ""
  }, {
      "id": "Alice7f69",
      "commit_time": 1594040060,
      "hash": "7f69500c35288c9ea60904f5c4d7091777c72bb1",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add chrome-public instance",
      "cl_url": ""
  }, {
      "id": "Iris98b7",
      "commit_time": 1594056570,
      "hash": "98b77200d1f7531ba5a15e86b4eec09b6e817872",
      "author": "Iris (iris@example.com)",
      "message": "[autoroll] Manually update cros afdo rollers",
      "cl_url": ""
  }, {
      "id": "Grace1324",
      "commit_time": 1594057104,
      "hash": "1324d5114c84dde45cb249edd3a96a156cc2dbcc",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add metrics-analysis-sk",
      "cl_url": ""
  }, {
      "id": "Bob8860",
      "commit_time": 1594057500,
      "hash": "8860a9911ef243baa630150af326bf862b25c92c",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port triage-page-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Alicefd65",
      "commit_time": 1594057737,
      "hash": "fd65c33dd11ac0b3232e97656ff89c2cf3037fc8",
      "author": "Alice (alice@example.com)",
      "message": "[envoy] Remove some deprecated types/warnings",
      "cl_url": ""
  }, {
      "id": "Alicefe76",
      "commit_time": 1594057900,
      "hash": "fe7626a48f6db9493d6c56e37039eaf6e0f4e6f0",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add now label to force redeployment",
      "cl_url": ""
  }, {
      "id": "Aliceb95c",
      "commit_time": 1594057920,
      "hash": "b95c52ff254f1209bb10fe53d7c5d8be40229672",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Fix links for positive/negatives",
      "cl_url": ""
  }, {
      "id": "Bob3efa",
      "commit_time": 1594060343,
      "hash": "3efa59a7bdc4c427ab39cfb0c37862a8b449abaf",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Make flutter-flutter-perf.skia.org public.",
      "cl_url": ""
  }, {
      "id": "Bob5912",
      "commit_time": 1594069700,
      "hash": "5912bed81b5801c952dc4485e1a9538ea3d0a9e8",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Convert alert-page-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Grace88e7",
      "commit_time": 1594069851,
      "hash": "88e733d409bf8eb45c42794d4d31e788e0148c41",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add metrics_analysis page",
      "cl_url": ""
  }, {
      "id": "Liam0c96",
      "commit_time": 1594099551,
      "hash": "0c968ed09a69027f69054b7c54547552de23d2ad",
      "author": "Liam (liam@example.com)",
      "message": "Update Go Deps",
      "cl_url": ""
  }, {
      "id": "Bob29b6",
      "commit_time": 1594132524,
      "hash": "29b6ac5b5dda6eef17412f508c0e37b6683d25b9",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Follup-up fixes for triage-page-sk.",
      "cl_url": ""
  }, {
      "id": "Dana1ef",
      "commit_time": 1594142854,
      "hash": "a1ef758371f48f7b5c762f7cbf701312a985b2d1",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] New contacts/reviewers for depot tools -\u003e chromium roller",
      "cl_url": ""
  }, {
      "id": "Franke005",
      "commit_time": 1594143264,
      "hash": "e0050ef02d8b60ffccab9761c2b562eee2bd2328",
      "author": "Frank (frank@example.com)",
      "message": "[Autorollers] Add support for canaries",
      "cl_url": ""
  }, {
      "id": "Franke3fd",
      "commit_time": 1594144804,
      "hash": "e3fd453fe6eabd0406f3dc81d52125f3ea260d35",
      "author": "Frank (frank@example.com)",
      "message": "[Autoroller] Turn on manual rolls for dart-\u003eflutter",
      "cl_url": ""
  }, {
      "id": "Bob4fc6",
      "commit_time": 1594148746,
      "hash": "4fc63f24215cbab97f4c68efd91b2b9922858ac2",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Follow-up fix for alert-page-sk.",
      "cl_url": ""
  }, {
      "id": "Bobcab3",
      "commit_time": 1594149354,
      "hash": "cab388a0127c610756b23658bf6db3032066a43a",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Pert explore-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bob62a5",
      "commit_time": 1594155754,
      "hash": "62a5a39a9efc2910c12b5b234b4c362892423891",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port cluster-page-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bobf32f",
      "commit_time": 1594156854,
      "hash": "f32f3d0efed3ec22b22870c4e5f91e82afb72faa",
      "author": "Bob (bob@example.com)",
      "message": "[query-sk] Fix styling of the filter input.",
      "cl_url": ""
  }, {
      "id": "Bob73d6",
      "commit_time": 1594175214,
      "hash": "73d6a9cf2efd672628bba8905ae3a06c06308b36",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Port cluster-lastn-page-sk to TypeScript.",
      "cl_url": ""
  }, {
      "id": "Bobb5d0",
      "commit_time": 1594175334,
      "hash": "b5d0551908826a934caf03ed92e55d3be885a177",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Add theme-chooser-sk to perf-scaffold-sk.",
      "cl_url": ""
  }, {
      "id": "Grace7b22",
      "commit_time": 1594210298,
      "hash": "7b220d29fd618f448f811782fe9e65b43d0408b2",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add chromium-build-selector-sk",
      "cl_url": ""
  }, {
      "id": "Grace41ab",
      "commit_time": 1594210307,
      "hash": "41abdffdd2158690f0a2ffcf5c681306d81db9a4",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add capture-skps-sk element.",
      "cl_url": ""
  }, {
      "id": "Grace83a5",
      "commit_time": 1594210316,
      "hash": "83a501cfa80b9a855e50db7de50099b7002018d9",
      "author": "Grace (grace@example.com)",
      "message": "[ct] Add capture_skps page.",
      "cl_url": ""
  }, {
      "id": "Dan074c",
      "commit_time": 1594213855,
      "hash": "074cf9058409fa4b94192477bb12505497cc9fdb",
      "author": "Dan (dan@example.com)",
      "message": "[Autoroller] Update freetype-chromium merge paths.",
      "cl_url": ""
  }, {
      "id": "Frankfcce",
      "commit_time": 1594216065,
      "hash": "fcce5bb3db4b0d55cee6a28a0645209d259525d7",
      "author": "Frank (frank@example.com)",
      "message": "[Autoroller] Fix Dockerfile",
      "cl_url": ""
  }, {
      "id": "Frank363e",
      "commit_time": 1594223425,
      "hash": "363e801179b8ca8b0857cdf101901c1ceea7ee24",
      "author": "Frank (frank@example.com)",
      "message": "[Autoroll] Add Get method to manual/db.go",
      "cl_url": ""
  }, {
      "id": "Frank39bb",
      "commit_time": 1594224365,
      "hash": "39bbe4780f836930f07c9ca44aeefb43a5ef279f",
      "author": "Frank (frank@example.com)",
      "message": "[Autoroller] Add ability to skip emails",
      "cl_url": ""
  }, {
      "id": "Henry1b9f",
      "commit_time": 1594227415,
      "hash": "1b9f11dac68a05704a0e8e81a24979ee2e35b3f6",
      "author": "Henry (henry@example.com)",
      "message": "Adding additional sheriff to PGO autorollers",
      "cl_url": ""
  }, {
      "id": "Erin3203",
      "commit_time": 1594228285,
      "hash": "32030e34124d0d756b3fb08c5448e80627e7228a",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] Add PageObjectElement class.",
      "cl_url": ""
  }, {
      "id": "Alice03f9",
      "commit_time": 1594228545,
      "hash": "03f9117e14567b4214357ad8888c309c04b80a3c",
      "author": "Alice (alice@example.com)",
      "message": "[infra-sk] Remove double scrollbar",
      "cl_url": ""
  }, {
      "id": "Bob59f3",
      "commit_time": 1594238455,
      "hash": "59f3bd6a902847a9e8d7966cfb4c8fab035c7196",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix cluster-page-sk",
      "cl_url": ""
  }, {
      "id": "Boba03a",
      "commit_time": 1594240555,
      "hash": "a03af3aaa9a46992dbbddef7a7203e597c681513",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Add a local themes.scss.",
      "cl_url": ""
  }, {
      "id": "Bob6cff",
      "commit_time": 1594247945,
      "hash": "6cff6cc521b6f0ffb61c9c9c3615c59e0b3d9239",
      "author": "Bob (bob@example.com)",
      "message": "[infra-sk] Fix banding on paramset-sk.",
      "cl_url": ""
  }, {
      "id": "Erin63f1",
      "commit_time": 1594251545,
      "hash": "63f1e762b9d60dde6533a02f14980177c007ffdc",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] paramset-sk: Add a page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Erin5cc4",
      "commit_time": 1594251885,
      "hash": "5cc491ae48e022b1eae9a1ed450933653e90c6d9",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] query-values-sk: Add a page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Erin8b42",
      "commit_time": 1594251925,
      "hash": "8b426883d0fe3aa21ceeabbe076cc651f78e2ff9",
      "author": "Erin (erin@example.com)",
      "message": "[infra-sk] query-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Alicefd71",
      "commit_time": 1594298616,
      "hash": "fd71f156e6c1b554d6952b7124c4917970af74e5",
      "author": "Alice (alice@example.com)",
      "message": "[particles] Expose version info about canvaskit build",
      "cl_url": ""
  }, {
      "id": "Bob6ffa",
      "commit_time": 1594307189,
      "hash": "6ffacc77fc578b66c4f180d40928bb476d0dae3f",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix perf-scaffold and theme CSS.",
      "cl_url": ""
  }, {
      "id": "Bob66f7",
      "commit_time": 1594313319,
      "hash": "66f797e32f53640ddaa69c985d31062fac7c18b9",
      "author": "Bob (bob@example.com)",
      "message": "[perf][golden] Upgrade elements-sk to 3.3.4.",
      "cl_url": ""
  }, {
      "id": "Johnd743",
      "commit_time": 1594315079,
      "hash": "d743a7d95130b863da3ec485a5dbbc876cff64bc",
      "author": "John (john@example.com)",
      "message": "Update default effect on particles.skia.org",
      "cl_url": ""
  }, {
      "id": "Erinf5b4",
      "commit_time": 1594321229,
      "hash": "f5b48d60d9dcb3e58e0001b026ef380cb4a66a99",
      "author": "Erin (erin@example.com)",
      "message": "[gold] query-dialog-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Erin4ad3",
      "commit_time": 1594322219,
      "hash": "4ad3587da0164c4ddc2d6c9cec2e0a46e39d81f4",
      "author": "Erin (erin@example.com)",
      "message": "[gold] trace-filter-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Alice83cd",
      "commit_time": 1594322790,
      "hash": "83cd41fd102bcdb1915596dc05c2b484b805d63c",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add cluster-digests-sk element.",
      "cl_url": ""
  }, {
      "id": "Alice2815",
      "commit_time": 1594322790,
      "hash": "28157d9c4a77178f8a348868ea8f5c24e70b0f0c",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add helpers for dealing with URL params to search-controls-sk",
      "cl_url": ""
  }, {
      "id": "Erinb44c",
      "commit_time": 1594323299,
      "hash": "b44c941764aebd6ccb47122f47f868e6e4f3955f",
      "author": "Erin (erin@example.com)",
      "message": "[gold] filter-dialog-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Erin2a25",
      "commit_time": 1594325479,
      "hash": "2a251850405ca966dda93bc1d481396e889ae38b",
      "author": "Erin (erin@example.com)",
      "message": "[gold] corpus-selector-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Bobed26",
      "commit_time": 1594325577,
      "hash": "ed267d27ef91d51d66032783cf5e1f295e5967e2",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Style the scrollbars.",
      "cl_url": ""
  }, {
      "id": "Bob9acc",
      "commit_time": 1594329579,
      "hash": "9acc96b0753c64413a742cae128f08582ace6968",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Don't display LSE where it doesn't apply.",
      "cl_url": ""
  }, {
      "id": "Frank1edb",
      "commit_time": 1594387930,
      "hash": "1edb3316298fd299eb2a74402557dc3db413cda8",
      "author": "Frank (frank@example.com)",
      "message": "[Autorollers] Do not add reviews to repo upload if no emails specified",
      "cl_url": ""
  }, {
      "id": "Boba7fd",
      "commit_time": 1594392714,
      "hash": "a7fd6c7ec66b93c5ea0c79f6ba5c401e317a95ce",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Follow-up for scrollbar styling.",
      "cl_url": ""
  }, {
      "id": "Alice829d",
      "commit_time": 1594392872,
      "hash": "829d9acd9d58e7ced801e208b5dc9342a1e47706",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Add click handlers to cluster-digests-sk",
      "cl_url": ""
  }, {
      "id": "Bob1161",
      "commit_time": 1594394000,
      "hash": "11610204ae961e4722262ef17afee82926f8571f",
      "author": "Bob (bob@example.com)",
      "message": "NewDataFrameIterator",
      "cl_url": ""
  }, {
      "id": "Bobe24d",
      "commit_time": 1594394300,
      "hash": "e24d7a62c06918bd7a395d65a027542423d73174",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Make cluster-page and lastn-page puppeteer tests deterministic.",
      "cl_url": ""
  }, {
      "id": "Bobf165",
      "commit_time": 1594398630,
      "hash": "f165a841a32d0d100be6a7053265b6fdc4540a67",
      "author": "Bob (bob@example.com)",
      "message": "[perf] cluster-summary2-sk show commit details for xbar.",
      "cl_url": ""
  }, {
      "id": "Katy574b",
      "commit_time": 1594408381,
      "hash": "574bd22f7107a7c3b80255210463286a66427d8e",
      "author": "Katy (katy@example.com)",
      "message": "Update sebmarchand's email address for the PGO profile rolls",
      "cl_url": ""
  }, {
      "id": "Aliceb26a",
      "commit_time": 1594409170,
      "hash": "b26acb953c927b50e6283b089cf9dcbf45a8793c",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Inclusive language",
      "cl_url": ""
  }, {
      "id": "Bob7869",
      "commit_time": 1594415761,
      "hash": "78696c00978453d3b835a6682c82bf02a62de575",
      "author": "Bob (bob@example.com)",
      "message": "[perf] LSE calc for OriginalStep is wrong.",
      "cl_url": ""
  }, {
      "id": "Boba615",
      "commit_time": 1594416021,
      "hash": "a615c7531284211220d675314c4c3cf61f61052c",
      "author": "Bob (bob@example.com)",
      "message": "[infra-sk] Move the fast filter on query-sk.",
      "cl_url": ""
  }, {
      "id": "Bob31d5",
      "commit_time": 1594476795,
      "hash": "31d53aac2f8f03a88993afc081282b75a6c94004",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Have ./modules/json/v1/index.ts have proper dependencies.",
      "cl_url": ""
  }, {
      "id": "Liamd749",
      "commit_time": 1594531254,
      "hash": "d749d74a95efc781ce0bb017d2a4b127621dff94",
      "author": "Liam (liam@example.com)",
      "message": "Update CIPD Packages",
      "cl_url": ""
  }, {
      "id": "Bobd355",
      "commit_time": 1594560830,
      "hash": "d3550732bff319d9b294b0081fc5cb9cfea81a62",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix explore query and skp markers.",
      "cl_url": ""
  }, {
      "id": "Bob4a60",
      "commit_time": 1594561923,
      "hash": "4a603f29e184ed0328eab94cbadf3f0ff967eb23",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Fix sidebar",
      "cl_url": ""
  }, {
      "id": "Erinc55e",
      "commit_time": 1594577125,
      "hash": "c55e15d570b1572a4384c4d537b1bf58123822d7",
      "author": "Erin (erin@example.com)",
      "message": "[gold] search-controls-sk: Add page object and use it in tests.",
      "cl_url": ""
  }, {
      "id": "Erin2dba",
      "commit_time": 1594578745,
      "hash": "2dba6c85dedc95ec7f51ee28191fd57d8426878b",
      "author": "Erin (erin@example.com)",
      "message": "[gold] Deflake gold_query-dialog-sk_multiple-values-selected.",
      "cl_url": ""
  }, {
      "id": "Bobd2cf",
      "commit_time": 1594641309,
      "hash": "d2cf0ee78159f379d506c12ecb13179b767205fa",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Remove Zoom Range button.",
      "cl_url": ""
  }, {
      "id": "Bob2a15",
      "commit_time": 1594641350,
      "hash": "2a155d19f97622052ad6b54e93178739a26cc99d",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Style fixes for inputs and buttons.",
      "cl_url": ""
  }, {
      "id": "Bob36d1",
      "commit_time": 1594641376,
      "hash": "36d176536e1725bd6d138ee977096bc240a1c4fa",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Actually load the Roboto font.",
      "cl_url": ""
  }, {
      "id": "Bobe951",
      "commit_time": 1594641517,
      "hash": "e9512257b55ada3101ae3b4a165cfb3ba94c8f5b",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Disable details tab on explore when disabled.",
      "cl_url": ""
  }, {
      "id": "Dan43b7",
      "commit_time": 1594645203,
      "hash": "43b728cf27f09e0fa285a8c0214050802e7644d5",
      "author": "Dan (dan@example.com)",
      "message": "Fix some non-inclusive language",
      "cl_url": ""
  }, {
      "id": "Frank2630",
      "commit_time": 1594647333,
      "hash": "2630055acc2f94f5002611c6262604b483b054df",
      "author": "Frank (frank@example.com)",
      "message": "[Autorollers] Do not \"wait\" for tree-status checks during dry runs",
      "cl_url": ""
  }, {
      "id": "Bobd4a9",
      "commit_time": 1594652733,
      "hash": "d4a9962e2adc2d0aa9c2324414f84721fc8b6da4",
      "author": "Bob (bob@example.com)",
      "message": "[perf] More consolidation on things that are commit numbers.",
      "cl_url": ""
  }, {
      "id": "Bob5be4",
      "commit_time": 1594654373,
      "hash": "5be44e3348441b2255ad692c23b7ac4e8cb94be0",
      "author": "Bob (bob@example.com)",
      "message": "[am] Fix double scrollbars.",
      "cl_url": ""
  }, {
      "id": "Bob8c43",
      "commit_time": 1594658786,
      "hash": "8c43148a19ca347041bc159ae775f7890dcce4e9",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Tighten up buttons.",
      "cl_url": ""
  }, {
      "id": "Bobbd7b",
      "commit_time": 1594661722,
      "hash": "bd7b96721c4294fceb165fbc76ef249204afa71d",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Tighten up buttons.",
      "cl_url": ""
  }, {
      "id": "Iris1e87",
      "commit_time": 1594664993,
      "hash": "1e873b7fb3987130f65036d6d725646250d89f39",
      "author": "Iris (iris@example.com)",
      "message": "[autoroll] Manually update cros orderfile rollers",
      "cl_url": ""
  }, {
      "id": "Dan88ff",
      "commit_time": 1594666443,
      "hash": "88ffb83541286cef05309cd97de2ca1f8c1a8202",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] Add URL to fake commit message data",
      "cl_url": ""
  }, {
      "id": "Erin01a7",
      "commit_time": 1594667253,
      "hash": "01a7d83dcf27c85965239edd20e43ff32a7203f5",
      "author": "Erin (erin@example.com)",
      "message": "Update go2ts to v.1.3.2.",
      "cl_url": ""
  }, {
      "id": "Erind1da",
      "commit_time": 1594667273,
      "hash": "d1daff06b038e9f0f37b2b0c6e62491a8e7a1c21",
      "author": "Erin (erin@example.com)",
      "message": "[gold] Add code generator for TypeScript RPC types.",
      "cl_url": ""
  }, {
      "id": "Bob7573",
      "commit_time": 1594735088,
      "hash": "757374e852412ae69194669c5d32a6bfc59486c6",
      "author": "Bob (bob@example.com)",
      "message": "[perf] Hide buttons that apply to highlighted traces.",
      "cl_url": ""
  }, {
      "id": "Alice6227",
      "commit_time": 1594749882,
      "hash": "6227f9af3ae06637a08db370d6725ae4ae27b11f",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Use proper templates for CL messages",
      "cl_url": ""
  }, {
      "id": "Aliceddec",
      "commit_time": 1594752126,
      "hash": "ddec7447f21cafd641614e65ebe3ecb88749ef00",
      "author": "Alice (alice@example.com)",
      "message": "[infra-sk] Move setQueryString to test_util",
      "cl_url": ""
  }, {
      "id": "Alicecfc3",
      "commit_time": 1594752126,
      "hash": "cfc32e173bb45a0d30c22e2c3e7066073aa40288",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Initial cluster-page-sk",
      "cl_url": ""
  }, {
      "id": "Dan553c",
      "commit_time": 1594752654,
      "hash": "553c6232549653161692528627dc875883987590",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] Longer display revision for CIPD packages",
      "cl_url": ""
  }, {
      "id": "Dan1cdb",
      "commit_time": 1594752784,
      "hash": "1cdb9a55b55a330dbaaed7c47fd6c913a0527510",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] Migrate to Lit-HTML and TypeScript",
      "cl_url": ""
  }, {
      "id": "Bob7097",
      "commit_time": 1594753677,
      "hash": "70979f673e3693fa15fe7fd427e1f9f6943318fc",
      "author": "Bob (bob@example.com)",
      "message": "[machine] Start recording when bot_config started running.",
      "cl_url": ""
  }, {
      "id": "Bob1595",
      "commit_time": 1594757287,
      "hash": "15957372ea630f7f2467c1db89a775323478dc12",
      "author": "Bob (bob@example.com)",
      "message": "Revert \"[named-fiddles] Remove named-fiddles.\"",
      "cl_url": ""
  }, {
      "id": "Alice7cc8",
      "commit_time": 1594757497,
      "hash": "7cc820a90366ff31415813c0881503325b9ed553",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Remove need for goldctl's service account to have read access.",
      "cl_url": ""
  }, {
      "id": "Bobb596",
      "commit_time": 1594758085,
      "hash": "b59652b6cfb3ce26053048486d0f0b922fe067f6",
      "author": "Bob (bob@example.com)",
      "message": "[named-fiddles] Fix release and alert message.",
      "cl_url": ""
  }, {
      "id": "Aliceda6a",
      "commit_time": 1594758914,
      "hash": "da6ab914994ad85537fe236a15e2762f89d497aa",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Update with clean image",
      "cl_url": ""
  }, {
      "id": "Frankbc87",
      "commit_time": 1594760687,
      "hash": "bc871511ec55f419c429fbee4edebddb7f622bf3",
      "author": "Frank (frank@example.com)",
      "message": "[Autorollers] When rev is a patch ref use gclient's patch-ref args",
      "cl_url": ""
  }, {
      "id": "Alicefc9b",
      "commit_time": 1594762977,
      "hash": "fc9bf78ccfe261f03094c4ba5558a160b80e499b",
      "author": "Alice (alice@example.com)",
      "message": "[gold] Remove unused executables",
      "cl_url": ""
  }, {
      "id": "Bobd9a6",
      "commit_time": 1594764007,
      "hash": "d9a61cf86f56e12924fef7b5aeaa63fb3963f781",
      "author": "Bob (bob@example.com)",
      "message": "[machine] Schedule pods for restart if they get too old.",
      "cl_url": ""
  }, {
      "id": "Dan251e",
      "commit_time": 1594812819,
      "hash": "251e0be180076aac9c9ab2964cea6d88f78023e2",
      "author": "Dan (dan@example.com)",
      "message": "Fix Infra-PerCommit-Build",
      "cl_url": ""
  }, {
      "id": "Dan3e53",
      "commit_time": 1594817699,
      "hash": "3e53cd963f319a3e3e293bd091e83149eab703f6",
      "author": "Dan (dan@example.com)",
      "message": "[autoroll] Fixes",
      "cl_url": ""
  }],
  "bulk_triage_data": {
      "gold_details-page-sk": {
          "29f31f703510c2091840b5cf2b032f56": "positive",
          "7c0a393e57f14b5372ec1590b79bed0f": "positive",
          "971fe90fa07ebc2c7d0c1a109a0f697c": "positive",
          "e49c92a2cff48531810cc5e863fad0ee": "positive"
      },
      "gold_search-controls-sk_right-hand-trace-filter-editor": {
          "5d8c80eda80e015d633a4125ab0232dc": "positive",
          "d20f37006e436fe17f50ecf49ff2bdb5": "positive",
          "fbd3de3fff6b852ae0bb6751b9763d27": "positive"
      },
      "perf_alert-config-sk": {
          "2fa58aa430e9c815755624ca6cca4a72": "negative",
          "ed4a8cf9ea9fbb57bf1f302537e07572": "untriaged"
      },
  }
};
