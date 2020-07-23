import { SearchResponse, StatusResponse, ParamSetResponse, ChangeListSummaryResponse, SearchResult } from '../rpc_types';

// Taken from https://skia-infra-gold.skia.org/json/trstatus on 2020-07-15.
export const statusResponse: StatusResponse = {
  "ok": false,
  "firstCommit": {
      "commit_time": 1592422850,
      "hash": "915a4938104e09e50b0f148220436ee9dfe3148e",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Port list-page-sk to lit-html",
      "is_cl": false
  },
  "lastCommit": {
      "commit_time": 1594817699,
      "hash": "3e53cd963f319a3e3e293bd091e83149eab703f6",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Fixes",
      "is_cl": false
  },
  "totalCommits": 213,
  "filledCommits": 200,
  "corpStatus": [{
      "name": "infra",
      "ok": false,
      "minCommitHash": "",
      "untriagedCount": 18,
      "negativeCount": 0
  }]
};

// Taken from https://skia-infra-gold.skia.org/json/paramset on 2020-07-15.
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

// Taken from https://skia-infra-gold.skia.org/json/search on 2020-07-15.
export const searchResponse: SearchResponse = {
  "digests": [{
      "digest": "fbd3de3fff6b852ae0bb6751b9763d27",
      "test": "gold_search-controls-sk_right-hand-trace-filter-editor",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
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
              "status": "untriaged"
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
              "diffs": {
                  "combined": 0.0921628,
                  "percent": 0.25010416,
                  "pixel": 2401
              },
              "digest": "5d8c80eda80e015d633a4125ab0232dc",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "2fa58aa430e9c815755624ca6cca4a72",
      "test": "perf_alert-config-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alert-config-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
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
              "status": "positive"
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
              "diffs": {
                  "combined": 0.11146385,
                  "percent": 0.110857144,
                  "pixel": 3880
              },
              "digest": "ed4a8cf9ea9fbb57bf1f302537e07572",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alert-config-sk"],
                  "source_type": ["infra"]
              },
              "n": 1
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ed4a8cf9ea9fbb57bf1f302537e07572",
      "test": "perf_alert-config-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T15:06:59.585035Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alert-config-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
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
              "status": "positive"
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
              "status": "positive"
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
              "diffs": {
                  "combined": 0.11146385,
                  "percent": 0.110857144,
                  "pixel": 3880
              },
              "digest": "2fa58aa430e9c815755624ca6cca4a72",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alert-config-sk"],
                  "source_type": ["infra"]
              },
              "n": 24
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "5c92542f830edf0474f72673d5da6a15",
      "test": "gold_list-page-sk_query-dialog",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_list-page-sk_query-dialog"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_list-page-sk_query-dialog,source_type=infra,",
              "data": [4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 6, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_list-page-sk_query-dialog",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "5c92542f830edf0474f72673d5da6a15",
              "status": "untriaged"
          }, {
              "digest": "fa440d95c9faca0d2258bb45addee8f8",
              "status": "positive"
          }, {
              "digest": "f2eb2dd09401fd7d77c3eec3bc159c2f",
              "status": "positive"
          }, {
              "digest": "1caf1da205f7e4e855b1d20905ec4bee",
              "status": "positive"
          }, {
              "digest": "7bf2c0b90544db414809c1d667621b2e",
              "status": "positive"
          }, {
              "digest": "958575b3b07dd23cf76d252982dfca76",
              "status": "positive"
          }, {
              "digest": "fbe811feec2a7ec4f1dbcc68f8a3d5fb",
              "status": "untriaged"
          }],
          "total_digests": 7
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 7477,
              "pixelDiffPercent": 0.7477,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.15935251,
                  "percent": 0.7477,
                  "pixel": 7477
              },
              "digest": "1caf1da205f7e4e855b1d20905ec4bee",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_list-page-sk_query-dialog"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "51a4276aec7c922ded06d866d986bb10",
      "test": "gold_query-dialog-sk_key-and-value-selected",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_query-dialog-sk_key-and-value-selected"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_query-dialog-sk_key-and-value-selected,source_type=infra,",
              "data": [4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_query-dialog-sk_key-and-value-selected",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "51a4276aec7c922ded06d866d986bb10",
              "status": "untriaged"
          }, {
              "digest": "603ca14e11008eda297988e7d26326ef",
              "status": "positive"
          }, {
              "digest": "0307e2a9286fc2b8159d2ba3a16e9dc7",
              "status": "positive"
          }, {
              "digest": "785c21f202276bffb29eb29fa7afc952",
              "status": "positive"
          }, {
              "digest": "a56c4c78b3e2418b35f080a9a960383d",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 3649,
              "pixelDiffPercent": 0.7602083,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.16067989,
                  "percent": 0.7602083,
                  "pixel": 3649
              },
              "digest": "785c21f202276bffb29eb29fa7afc952",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_query-dialog-sk_key-and-value-selected"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "c0a25c6b2262c8c3346181f427327b84",
      "test": "gold_digest-details-sk_negative-only",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-06-18T14:03:21.060362Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_digest-details-sk_negative-only"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_digest-details-sk_negative-only,source_type=infra,",
              "data": [3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_digest-details-sk_negative-only",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "c0a25c6b2262c8c3346181f427327b84",
              "status": "positive"
          }, {
              "digest": "92137d4b89a7be781008094acc232308",
              "status": "positive"
          }, {
              "digest": "9931070715675310cabe7e26561b1896",
              "status": "untriaged"
          }, {
              "digest": "c2e28c0c694d45aa3684c5f16babeef0",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2492,
              "pixelDiffPercent": 0.5581312,
              "maxRGBADiffs": [27, 1, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.17756304,
                  "percent": 0.5581312,
                  "pixel": 2492
              },
              "digest": "c2e28c0c694d45aa3684c5f16babeef0",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_digest-details-sk_negative-only"],
                  "source_type": ["infra"]
              },
              "n": 4
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "c2e28c0c694d45aa3684c5f16babeef0",
      "test": "gold_digest-details-sk_negative-only",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-05-27T19:45:01.147062Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_digest-details-sk_negative-only"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_digest-details-sk_negative-only,source_type=infra,",
              "data": [0, 0, 0, 0, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_digest-details-sk_negative-only",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "c2e28c0c694d45aa3684c5f16babeef0",
              "status": "positive"
          }, {
              "digest": "92137d4b89a7be781008094acc232308",
              "status": "positive"
          }, {
              "digest": "9931070715675310cabe7e26561b1896",
              "status": "untriaged"
          }, {
              "digest": "c0a25c6b2262c8c3346181f427327b84",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2492,
              "pixelDiffPercent": 0.5581312,
              "maxRGBADiffs": [27, 1, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.17756304,
                  "percent": 0.5581312,
                  "pixel": 2492
              },
              "digest": "c0a25c6b2262c8c3346181f427327b84",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_digest-details-sk_negative-only"],
                  "source_type": ["infra"]
              },
              "n": 134
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "1bea9fc10636acf7ae5a2b3cc2c9ced6",
      "test": "gold_search-controls-sk_more-filters",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_more-filters"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_more-filters,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_more-filters",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "1bea9fc10636acf7ae5a2b3cc2c9ced6",
              "status": "positive"
          }, {
              "digest": "4d9f238c419a4342556a788718282fa7",
              "status": "untriaged"
          }, {
              "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 14108,
              "pixelDiffPercent": 1.4695834,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.1869138,
                  "percent": 1.4695834,
                  "pixel": 14108
              },
              "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_more-filters"],
                  "source_type": ["infra"]
              },
              "n": 77
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
      "test": "gold_search-controls-sk_more-filters",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-30T17:39:20.873232Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_more-filters"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_more-filters,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_more-filters",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
              "status": "positive"
          }, {
              "digest": "1bea9fc10636acf7ae5a2b3cc2c9ced6",
              "status": "positive"
          }, {
              "digest": "4d9f238c419a4342556a788718282fa7",
              "status": "untriaged"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 14108,
              "pixelDiffPercent": 1.4695834,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.1869138,
                  "percent": 1.4695834,
                  "pixel": 14108
              },
              "digest": "1bea9fc10636acf7ae5a2b3cc2c9ced6",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_more-filters"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "5d992befcadcaf7461ba435975fa11e4",
      "test": "infra-sk_expandable-textarea-sk_open",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_expandable-textarea-sk_open"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_expandable-textarea-sk_open,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_expandable-textarea-sk_open",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "5d992befcadcaf7461ba435975fa11e4",
              "status": "positive"
          }, {
              "digest": "78e249a7ad42ca936437a60673953b7b",
              "status": "positive"
          }, {
              "digest": "d0868fbcf15879c2c9f41d9f19c249df",
              "status": "negative"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": {
              "numDiffPixels": 166537,
              "pixelDiffPercent": 83.2685,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 1.4069693,
                  "percent": 83.2685,
                  "pixel": 166537
              },
              "digest": "d0868fbcf15879c2c9f41d9f19c249df",
              "status": "negative",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_expandable-textarea-sk_open"],
                  "source_type": ["infra"]
              },
              "n": 34
          },
          "pos": {
              "numDiffPixels": 3255,
              "pixelDiffPercent": 1.6275,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.19670017,
                  "percent": 1.6275,
                  "pixel": 3255
              },
              "digest": "78e249a7ad42ca936437a60673953b7b",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_expandable-textarea-sk_open"],
                  "source_type": ["infra"]
              },
              "n": 89
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "78e249a7ad42ca936437a60673953b7b",
      "test": "infra-sk_expandable-textarea-sk_open",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-06-24T21:31:07.604887Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_expandable-textarea-sk_open"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_expandable-textarea-sk_open,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_expandable-textarea-sk_open",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "78e249a7ad42ca936437a60673953b7b",
              "status": "positive"
          }, {
              "digest": "5d992befcadcaf7461ba435975fa11e4",
              "status": "positive"
          }, {
              "digest": "d0868fbcf15879c2c9f41d9f19c249df",
              "status": "negative"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": {
              "numDiffPixels": 163282,
              "pixelDiffPercent": 81.641,
              "maxRGBADiffs": [5, 5, 5, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 1.177428,
                  "percent": 81.641,
                  "pixel": 163282
              },
              "digest": "d0868fbcf15879c2c9f41d9f19c249df",
              "status": "negative",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_expandable-textarea-sk_open"],
                  "source_type": ["infra"]
              },
              "n": 34
          },
          "pos": {
              "numDiffPixels": 3255,
              "pixelDiffPercent": 1.6275,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.19670017,
                  "percent": 1.6275,
                  "pixel": 3255
              },
              "digest": "5d992befcadcaf7461ba435975fa11e4",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_expandable-textarea-sk_open"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "7a0f953d83033d05c017c0edca73d0c1",
      "test": "gold_trace-filter-sk_nonempty_query-dialog-open",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_trace-filter-sk_nonempty_query-dialog-open"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_trace-filter-sk_nonempty_query-dialog-open,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_trace-filter-sk_nonempty_query-dialog-open",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "7a0f953d83033d05c017c0edca73d0c1",
              "status": "untriaged"
          }, {
              "digest": "206feaa41b7e220d4c85f5d653205c84",
              "status": "positive"
          }, {
              "digest": "9ec3918a8a7e443c917f2c16a4c90e53",
              "status": "positive"
          }, {
              "digest": "d05a25337eb8393b9e31dd59c5199c9c",
              "status": "positive"
          }, {
              "digest": "ff4aecaa5b2500f2e8b214250299c0b8",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 6357,
              "pixelDiffPercent": 1.324375,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2120805,
                  "percent": 1.324375,
                  "pixel": 6357
              },
              "digest": "d05a25337eb8393b9e31dd59c5199c9c",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_trace-filter-sk_nonempty_query-dialog-open"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "a0d92dbe6b7cd6e7e9ee0f6f1e338082",
      "test": "gold_query-dialog-sk_nonempty-initial-selection",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_query-dialog-sk_nonempty-initial-selection"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_query-dialog-sk_nonempty-initial-selection,source_type=infra,",
              "data": [4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_query-dialog-sk_nonempty-initial-selection",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "a0d92dbe6b7cd6e7e9ee0f6f1e338082",
              "status": "untriaged"
          }, {
              "digest": "54768e7da1b1200942d2189056d6968b",
              "status": "positive"
          }, {
              "digest": "d4c89fc2c41e1077eb56903541c5de25",
              "status": "positive"
          }, {
              "digest": "c953f60c7302d06b157b3f88dd5624ed",
              "status": "positive"
          }, {
              "digest": "2ba56ee2dffbc6da9ed918683248b5eb",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 6357,
              "pixelDiffPercent": 1.324375,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2120805,
                  "percent": 1.324375,
                  "pixel": 6357
              },
              "digest": "c953f60c7302d06b157b3f88dd5624ed",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_query-dialog-sk_nonempty-initial-selection"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "4d9f238c419a4342556a788718282fa7",
      "test": "gold_search-controls-sk_more-filters",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_more-filters"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_more-filters,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_more-filters",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "4d9f238c419a4342556a788718282fa7",
              "status": "untriaged"
          }, {
              "digest": "1bea9fc10636acf7ae5a2b3cc2c9ced6",
              "status": "positive"
          }, {
              "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 14109,
              "pixelDiffPercent": 1.4696875,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.22341263,
                  "percent": 1.4696875,
                  "pixel": 14109
              },
              "digest": "ae8e679df1b416ca5bf1d793bca8e3dc",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_more-filters"],
                  "source_type": ["infra"]
              },
              "n": 77
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "a521d05ee320e76c740cd69c6a7ec0cb",
      "test": "gold_search-controls-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "a521d05ee320e76c740cd69c6a7ec0cb",
              "status": "positive"
          }, {
              "digest": "548551fe95eeb01bbbf3be044035b9c6",
              "status": "untriaged"
          }, {
              "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
              "status": "positive"
          }, {
              "digest": "f375370288ef134d4d18bc6712cf69aa",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 22615,
              "pixelDiffPercent": 2.355729,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.23665023,
                  "percent": 2.355729,
                  "pixel": 22615
              },
              "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk"],
                  "source_type": ["infra"]
              },
              "n": 77
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
      "test": "gold_search-controls-sk",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-30T17:39:23.10332Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
              "status": "positive"
          }, {
              "digest": "a521d05ee320e76c740cd69c6a7ec0cb",
              "status": "positive"
          }, {
              "digest": "548551fe95eeb01bbbf3be044035b9c6",
              "status": "untriaged"
          }, {
              "digest": "f375370288ef134d4d18bc6712cf69aa",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 22615,
              "pixelDiffPercent": 2.355729,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.23665023,
                  "percent": 2.355729,
                  "pixel": 22615
              },
              "digest": "a521d05ee320e76c740cd69c6a7ec0cb",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "7eac1eebcc2250813ae69f2fbf3fdefd",
      "test": "perf_alerts-page-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alerts-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_alerts-page-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 7, 5, 5, 5, 5, 5, 6, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_alerts-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "7eac1eebcc2250813ae69f2fbf3fdefd",
              "status": "untriaged"
          }, {
              "digest": "b627c19049d4430ca951971ce2850732",
              "status": "untriaged"
          }, {
              "digest": "aa697601f254247b9dd904c5efa6f132",
              "status": "untriaged"
          }, {
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive"
          }, {
              "digest": "081691a6a73bbaaf88a88926fd5e2228",
              "status": "positive"
          }, {
              "digest": "b05f062cdd0a52bc68a64c3cb3b0c808",
              "status": "untriaged"
          }, {
              "digest": "e7a14dd2617ddc77feeef0ec808a0e19",
              "status": "positive"
          }, {
              "digest": "5e6e7a68e4282bf03e9f50ab772850a1",
              "status": "untriaged"
          }],
          "total_digests": 8
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 1441,
              "pixelDiffPercent": 0.4503125,
              "maxRGBADiffs": [27, 63, 29, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2563479,
                  "percent": 0.4503125,
                  "pixel": 1441
              },
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alerts-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 27
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "239b815d1b1ea33454696a9e1e6e07f2",
      "test": "infra-sk_autogrow-textarea-sk_shrink",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-06-24T21:31:07.604887Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_shrink"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_shrink,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_shrink",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "239b815d1b1ea33454696a9e1e6e07f2",
              "status": "positive"
          }, {
              "digest": "e5010c288a98d383eb457157b608a8bf",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2344,
              "pixelDiffPercent": 0.48833334,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.25835082,
                  "percent": 0.48833334,
                  "pixel": 2344
              },
              "digest": "e5010c288a98d383eb457157b608a8bf",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_shrink"],
                  "source_type": ["infra"]
              },
              "n": 34
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "465ef8c65d3881403a6862168960752a",
      "test": "infra-sk_autogrow-textarea-sk_filled",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-23T19:53:18.372832Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_filled"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_filled,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_filled",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "465ef8c65d3881403a6862168960752a",
              "status": "positive"
          }, {
              "digest": "e774564e4b6443f0e95a080b0827ab56",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2344,
              "pixelDiffPercent": 0.48833334,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.25835082,
                  "percent": 0.48833334,
                  "pixel": 2344
              },
              "digest": "e774564e4b6443f0e95a080b0827ab56",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_filled"],
                  "source_type": ["infra"]
              },
              "n": 145
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e5010c288a98d383eb457157b608a8bf",
      "test": "infra-sk_autogrow-textarea-sk_shrink",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-23T19:53:21.934608Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_shrink"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_shrink,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_shrink",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e5010c288a98d383eb457157b608a8bf",
              "status": "positive"
          }, {
              "digest": "239b815d1b1ea33454696a9e1e6e07f2",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2344,
              "pixelDiffPercent": 0.48833334,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.25835082,
                  "percent": 0.48833334,
                  "pixel": 2344
              },
              "digest": "239b815d1b1ea33454696a9e1e6e07f2",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_shrink"],
                  "source_type": ["infra"]
              },
              "n": 145
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e774564e4b6443f0e95a080b0827ab56",
      "test": "infra-sk_autogrow-textarea-sk_filled",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-06-24T21:31:07.604887Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_filled"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_filled,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_filled",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e774564e4b6443f0e95a080b0827ab56",
              "status": "positive"
          }, {
              "digest": "465ef8c65d3881403a6862168960752a",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2344,
              "pixelDiffPercent": 0.48833334,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.25835082,
                  "percent": 0.48833334,
                  "pixel": 2344
              },
              "digest": "465ef8c65d3881403a6862168960752a",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_filled"],
                  "source_type": ["infra"]
              },
              "n": 34
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "5d8c80eda80e015d633a4125ab0232dc",
      "test": "gold_search-controls-sk_right-hand-trace-filter-editor",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-07T23:55:42.651867Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_right-hand-trace-filter-editor,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_right-hand-trace-filter-editor",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "5d8c80eda80e015d633a4125ab0232dc",
              "status": "positive"
          }, {
              "digest": "0b6e67b8c8123a3fce7f3a98ef0ea71d",
              "status": "positive"
          }, {
              "digest": "d20f37006e436fe17f50ecf49ff2bdb5",
              "status": "positive"
          }, {
              "digest": "fbd3de3fff6b852ae0bb6751b9763d27",
              "status": "untriaged"
          }, {
              "digest": "88aa1cdc50433c0ec4404485eeb63b69",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 27790,
              "pixelDiffPercent": 2.8947916,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.26233298,
                  "percent": 2.8947916,
                  "pixel": 27790
              },
              "digest": "d20f37006e436fe17f50ecf49ff2bdb5",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
                  "source_type": ["infra"]
              },
              "n": 19
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "d20f37006e436fe17f50ecf49ff2bdb5",
      "test": "gold_search-controls-sk_right-hand-trace-filter-editor",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_right-hand-trace-filter-editor,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_right-hand-trace-filter-editor",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "d20f37006e436fe17f50ecf49ff2bdb5",
              "status": "positive"
          }, {
              "digest": "0b6e67b8c8123a3fce7f3a98ef0ea71d",
              "status": "positive"
          }, {
              "digest": "fbd3de3fff6b852ae0bb6751b9763d27",
              "status": "untriaged"
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
              "numDiffPixels": 27790,
              "pixelDiffPercent": 2.8947916,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.26233298,
                  "percent": 2.8947916,
                  "pixel": 27790
              },
              "digest": "5d8c80eda80e015d633a4125ab0232dc",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_right-hand-trace-filter-editor"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "955cc67da667b7e93685f8bd70b6d0fa",
      "test": "gold_diff-page-sk",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-06-11T12:42:49.641532Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_diff-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_diff-page-sk,source_type=infra,",
              "data": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_diff-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "955cc67da667b7e93685f8bd70b6d0fa",
              "status": "positive"
          }, {
              "digest": "a16db3e2e228c78a0833da3e2939ae4d",
              "status": "positive"
          }, {
              "digest": "693d37373b6c349bcc8eb042b8b605fe",
              "status": "untriaged"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30404,
              "pixelDiffPercent": 3.3410988,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2818311,
                  "percent": 3.3410988,
                  "pixel": 30404
              },
              "digest": "a16db3e2e228c78a0833da3e2939ae4d",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_diff-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
      "test": "gold_details-page-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_details-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_details-page-sk,source_type=infra,",
              "data": [3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_details-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
              "status": "positive"
          }, {
              "digest": "29f31f703510c2091840b5cf2b032f56",
              "status": "untriaged"
          }, {
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive"
          }, {
              "digest": "7c0a393e57f14b5372ec1590b79bed0f",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30404,
              "pixelDiffPercent": 3.3410988,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2818311,
                  "percent": 3.3410988,
                  "pixel": 30404
              },
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_details-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 134
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "a16db3e2e228c78a0833da3e2939ae4d",
      "test": "gold_diff-page-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_diff-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_diff-page-sk,source_type=infra,",
              "data": [2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_diff-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "a16db3e2e228c78a0833da3e2939ae4d",
              "status": "positive"
          }, {
              "digest": "693d37373b6c349bcc8eb042b8b605fe",
              "status": "untriaged"
          }, {
              "digest": "955cc67da667b7e93685f8bd70b6d0fa",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30404,
              "pixelDiffPercent": 3.3410988,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2818311,
                  "percent": 3.3410988,
                  "pixel": 30404
              },
              "digest": "955cc67da667b7e93685f8bd70b6d0fa",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_diff-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 138
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e49c92a2cff48531810cc5e863fad0ee",
      "test": "gold_details-page-sk",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-06-18T14:03:12.412279Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_details-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_details-page-sk,source_type=infra,",
              "data": [3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_details-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive"
          }, {
              "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
              "status": "positive"
          }, {
              "digest": "29f31f703510c2091840b5cf2b032f56",
              "status": "untriaged"
          }, {
              "digest": "7c0a393e57f14b5372ec1590b79bed0f",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30404,
              "pixelDiffPercent": 3.3410988,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2818311,
                  "percent": 3.3410988,
                  "pixel": 30404
              },
              "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_details-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "548551fe95eeb01bbbf3be044035b9c6",
      "test": "gold_search-controls-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "548551fe95eeb01bbbf3be044035b9c6",
              "status": "untriaged"
          }, {
              "digest": "a521d05ee320e76c740cd69c6a7ec0cb",
              "status": "positive"
          }, {
              "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
              "status": "positive"
          }, {
              "digest": "f375370288ef134d4d18bc6712cf69aa",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 22623,
              "pixelDiffPercent": 2.3565626,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.28290114,
                  "percent": 2.3565626,
                  "pixel": 22623
              },
              "digest": "e49b6e6e3f06ac9866b6602e6a98e910",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk"],
                  "source_type": ["infra"]
              },
              "n": 77
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "d313a417d34dde8093aba66b964e8d6e",
      "test": "infra-sk_autogrow-textarea-sk_grow",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-23T19:53:20.795534Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_grow"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_grow,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_grow",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "d313a417d34dde8093aba66b964e8d6e",
              "status": "positive"
          }, {
              "digest": "e70554b623c7ca35ae72320f1d8c510e",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2824,
              "pixelDiffPercent": 0.5883333,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.28357202,
                  "percent": 0.5883333,
                  "pixel": 2824
              },
              "digest": "e70554b623c7ca35ae72320f1d8c510e",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_grow"],
                  "source_type": ["infra"]
              },
              "n": 145
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e70554b623c7ca35ae72320f1d8c510e",
      "test": "infra-sk_autogrow-textarea-sk_grow",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-06-24T21:31:07.604887Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["infra-sk_autogrow-textarea-sk_grow"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=infra-sk_autogrow-textarea-sk_grow,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "infra-sk_autogrow-textarea-sk_grow",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e70554b623c7ca35ae72320f1d8c510e",
              "status": "positive"
          }, {
              "digest": "d313a417d34dde8093aba66b964e8d6e",
              "status": "positive"
          }],
          "total_digests": 2
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2824,
              "pixelDiffPercent": 0.5883333,
              "maxRGBADiffs": [61, 33, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.28357202,
                  "percent": 0.5883333,
                  "pixel": 2824
              },
              "digest": "d313a417d34dde8093aba66b964e8d6e",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["infra-sk_autogrow-textarea-sk_grow"],
                  "source_type": ["infra"]
              },
              "n": 34
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "5aa63a6fa321ba696c8f47c4758a10d3",
      "test": "gold_ignores-page-sk_create-modal",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_ignores-page-sk_create-modal"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_ignores-page-sk_create-modal,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_ignores-page-sk_create-modal",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "5aa63a6fa321ba696c8f47c4758a10d3",
              "status": "positive"
          }, {
              "digest": "4c4311a96652325ed782eb0251b07520",
              "status": "untriaged"
          }, {
              "digest": "ea340245f31b40f23bb6d255d504db7a",
              "status": "positive"
          }, {
              "digest": "19c7ff387e591ddf467afaa4c734873d",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 61560,
              "pixelDiffPercent": 3.6426036,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29427287,
                  "percent": 3.6426036,
                  "pixel": 61560
              },
              "digest": "ea340245f31b40f23bb6d255d504db7a",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_ignores-page-sk_create-modal"],
                  "source_type": ["infra"]
              },
              "n": 21
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ea340245f31b40f23bb6d255d504db7a",
      "test": "gold_ignores-page-sk_create-modal",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-07T23:55:49.304489Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_ignores-page-sk_create-modal"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_ignores-page-sk_create-modal,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_ignores-page-sk_create-modal",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "ea340245f31b40f23bb6d255d504db7a",
              "status": "positive"
          }, {
              "digest": "4c4311a96652325ed782eb0251b07520",
              "status": "untriaged"
          }, {
              "digest": "5aa63a6fa321ba696c8f47c4758a10d3",
              "status": "positive"
          }, {
              "digest": "19c7ff387e591ddf467afaa4c734873d",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 61560,
              "pixelDiffPercent": 3.6426036,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29427287,
                  "percent": 3.6426036,
                  "pixel": 61560
              },
              "digest": "5aa63a6fa321ba696c8f47c4758a10d3",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_ignores-page-sk_create-modal"],
                  "source_type": ["infra"]
              },
              "n": 19
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "880cbde400d0f3c6b864afa3e1397fe0",
      "test": "perf_day-range-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T15:06:59.585035Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_day-range-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_day-range-sk,source_type=infra,",
              "data": [6, 6, 6, 6, 6, 6, 6, 6, 6, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 3, 3, 3, 3, 3, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_day-range-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "880cbde400d0f3c6b864afa3e1397fe0",
              "status": "positive"
          }, {
              "digest": "e065e221cbd63a5a1a495c985afafe8d",
              "status": "positive"
          }, {
              "digest": "1c340a55382ddc351ca9db19ef0838b9",
              "status": "untriaged"
          }, {
              "digest": "e8bbd7390e4e75c5dc64e2d28ff14271",
              "status": "untriaged"
          }, {
              "digest": "a04093e446a66ac846f21e57ee2dd09f",
              "status": "positive"
          }, {
              "digest": "bb631f18afdea2df8643df4802eb23e8",
              "status": "positive"
          }, {
              "digest": "b84e67fd37083e7440a2cf12a0961426",
              "status": "positive"
          }],
          "total_digests": 7
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 8053,
              "pixelDiffPercent": 3.6604545,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29499304,
                  "percent": 3.6604545,
                  "pixel": 8053
              },
              "digest": "e065e221cbd63a5a1a495c985afafe8d",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_day-range-sk"],
                  "source_type": ["infra"]
              },
              "n": 48
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "e065e221cbd63a5a1a495c985afafe8d",
      "test": "perf_day-range-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_day-range-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_day-range-sk,source_type=infra,",
              "data": [6, 6, 6, 6, 6, 6, 6, 6, 6, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "perf_day-range-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "e065e221cbd63a5a1a495c985afafe8d",
              "status": "positive"
          }, {
              "digest": "1c340a55382ddc351ca9db19ef0838b9",
              "status": "untriaged"
          }, {
              "digest": "e8bbd7390e4e75c5dc64e2d28ff14271",
              "status": "untriaged"
          }, {
              "digest": "880cbde400d0f3c6b864afa3e1397fe0",
              "status": "positive"
          }, {
              "digest": "a04093e446a66ac846f21e57ee2dd09f",
              "status": "positive"
          }, {
              "digest": "bb631f18afdea2df8643df4802eb23e8",
              "status": "positive"
          }, {
              "digest": "b84e67fd37083e7440a2cf12a0961426",
              "status": "positive"
          }],
          "total_digests": 7
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 8053,
              "pixelDiffPercent": 3.6604545,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29499304,
                  "percent": 3.6604545,
                  "pixel": 8053
              },
              "digest": "880cbde400d0f3c6b864afa3e1397fe0",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_day-range-sk"],
                  "source_type": ["infra"]
              },
              "n": 1
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "b608d6f2b679db35e4229a8ea1246d92",
      "test": "gold_ignores-page-sk_update-modal",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-07T23:55:51.586633Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_ignores-page-sk_update-modal"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_ignores-page-sk_update-modal,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_ignores-page-sk_update-modal",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "b608d6f2b679db35e4229a8ea1246d92",
              "status": "positive"
          }, {
              "digest": "6aff3f28945dcdaf1120196595e81232",
              "status": "untriaged"
          }, {
              "digest": "fb292ed5d86c6305cd62c247a7a5abc7",
              "status": "positive"
          }, {
              "digest": "0ac41100340fa18d3ebbe8ea049c8066",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 61904,
              "pixelDiffPercent": 3.6629586,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29509392,
                  "percent": 3.6629586,
                  "pixel": 61904
              },
              "digest": "fb292ed5d86c6305cd62c247a7a5abc7",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_ignores-page-sk_update-modal"],
                  "source_type": ["infra"]
              },
              "n": 19
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "fb292ed5d86c6305cd62c247a7a5abc7",
      "test": "gold_ignores-page-sk_update-modal",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_ignores-page-sk_update-modal"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_ignores-page-sk_update-modal,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_ignores-page-sk_update-modal",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "fb292ed5d86c6305cd62c247a7a5abc7",
              "status": "positive"
          }, {
              "digest": "6aff3f28945dcdaf1120196595e81232",
              "status": "untriaged"
          }, {
              "digest": "b608d6f2b679db35e4229a8ea1246d92",
              "status": "positive"
          }, {
              "digest": "0ac41100340fa18d3ebbe8ea049c8066",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 61904,
              "pixelDiffPercent": 3.6629586,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.29509392,
                  "percent": 3.6629586,
                  "pixel": 61904
              },
              "digest": "b608d6f2b679db35e4229a8ea1246d92",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_ignores-page-sk_update-modal"],
                  "source_type": ["infra"]
              },
              "n": 21
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "45f79b03570eb6e2d85032ae65ae77fe",
      "test": "perf_query-chooser-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_query-chooser-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_query-chooser-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 7, 7, 7, 7, 7, 7, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 6, 6, 6, 6, 6, 6, 6, 3, 3, 3, 3, 3, 3, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_query-chooser-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "45f79b03570eb6e2d85032ae65ae77fe",
              "status": "positive"
          }, {
              "digest": "da43dc5c269788da252bc2d172394d64",
              "status": "untriaged"
          }, {
              "digest": "7e52b50a811dc11ce746cb7d7b98b059",
              "status": "untriaged"
          }, {
              "digest": "c890aae123627fdb7e2f158e3167c775",
              "status": "untriaged"
          }, {
              "digest": "2b2f06930a92a6d27439062c7b4f76c2",
              "status": "positive"
          }, {
              "digest": "2bc7eb220d3c2e6e821dd242fd4ae01a",
              "status": "positive"
          }, {
              "digest": "63aed096af9ee5f92ac440822523fd6b",
              "status": "positive"
          }, {
              "digest": "26611c1e731d57811b862aa459b0ea79",
              "status": "positive"
          }, {
              "digest": "772b2d192a55acb5dc9369bda124bd19",
              "status": "positive"
          }],
          "total_digests": 9
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 22615,
              "pixelDiffPercent": 3.7691667,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2993415,
                  "percent": 3.7691667,
                  "pixel": 22615
              },
              "digest": "772b2d192a55acb5dc9369bda124bd19",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_query-chooser-sk"],
                  "source_type": ["infra"]
              },
              "n": 1
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "772b2d192a55acb5dc9369bda124bd19",
      "test": "perf_query-chooser-sk",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T15:06:59.585035Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["perf_query-chooser-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_query-chooser-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 8, 8, 8, 8, 8, 8, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 3, 3, 3, 3, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_query-chooser-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "772b2d192a55acb5dc9369bda124bd19",
              "status": "positive"
          }, {
              "digest": "da43dc5c269788da252bc2d172394d64",
              "status": "untriaged"
          }, {
              "digest": "7e52b50a811dc11ce746cb7d7b98b059",
              "status": "untriaged"
          }, {
              "digest": "c890aae123627fdb7e2f158e3167c775",
              "status": "untriaged"
          }, {
              "digest": "2b2f06930a92a6d27439062c7b4f76c2",
              "status": "positive"
          }, {
              "digest": "45f79b03570eb6e2d85032ae65ae77fe",
              "status": "positive"
          }, {
              "digest": "2bc7eb220d3c2e6e821dd242fd4ae01a",
              "status": "positive"
          }, {
              "digest": "63aed096af9ee5f92ac440822523fd6b",
              "status": "positive"
          }, {
              "digest": "26611c1e731d57811b862aa459b0ea79",
              "status": "positive"
          }],
          "total_digests": 9
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 22615,
              "pixelDiffPercent": 3.7691667,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.2993415,
                  "percent": 3.7691667,
                  "pixel": 22615
              },
              "digest": "45f79b03570eb6e2d85032ae65ae77fe",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_query-chooser-sk"],
                  "source_type": ["infra"]
              },
              "n": 19
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "b627c19049d4430ca951971ce2850732",
      "test": "perf_alerts-page-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alerts-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_alerts-page-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 7, 5, 5, 5, 5, 5, 6, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "perf_alerts-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "b627c19049d4430ca951971ce2850732",
              "status": "untriaged"
          }, {
              "digest": "aa697601f254247b9dd904c5efa6f132",
              "status": "untriaged"
          }, {
              "digest": "7eac1eebcc2250813ae69f2fbf3fdefd",
              "status": "untriaged"
          }, {
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive"
          }, {
              "digest": "081691a6a73bbaaf88a88926fd5e2228",
              "status": "positive"
          }, {
              "digest": "b05f062cdd0a52bc68a64c3cb3b0c808",
              "status": "untriaged"
          }, {
              "digest": "e7a14dd2617ddc77feeef0ec808a0e19",
              "status": "positive"
          }, {
              "digest": "5e6e7a68e4282bf03e9f50ab772850a1",
              "status": "untriaged"
          }],
          "total_digests": 8
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2025,
              "pixelDiffPercent": 0.6328125,
              "maxRGBADiffs": [27, 63, 29, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.3038857,
                  "percent": 0.6328125,
                  "pixel": 2025
              },
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alerts-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 27
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "2e1a88b33a407e1f2254e58d02ee0057",
      "test": "gold_search-controls-sk_left-hand-trace-filter-editor",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_search-controls-sk_left-hand-trace-filter-editor"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_search-controls-sk_left-hand-trace-filter-editor,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_search-controls-sk_left-hand-trace-filter-editor",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "2e1a88b33a407e1f2254e58d02ee0057",
              "status": "untriaged"
          }, {
              "digest": "a379e64d78e3ba0fa1004114fb50c4f9",
              "status": "positive"
          }, {
              "digest": "3f3d694aa19033bee7a1219511768aa8",
              "status": "positive"
          }, {
              "digest": "cc5bc3c84cf70a8e109f591b7cf39ccb",
              "status": "positive"
          }, {
              "digest": "c56a8535c4cf6b87e51cc690f0292b68",
              "status": "positive"
          }],
          "total_digests": 5
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 26401,
              "pixelDiffPercent": 2.7501042,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.30561158,
                  "percent": 2.7501042,
                  "pixel": 26401
              },
              "digest": "cc5bc3c84cf70a8e109f591b7cf39ccb",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_search-controls-sk_left-hand-trace-filter-editor"],
                  "source_type": ["infra"]
              },
              "n": 15
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "aa697601f254247b9dd904c5efa6f132",
      "test": "perf_alerts-page-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["perf_alerts-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=perf_alerts-page-sk,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 7, 5, 5, 5, 5, 5, 6, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "perf_alerts-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "aa697601f254247b9dd904c5efa6f132",
              "status": "untriaged"
          }, {
              "digest": "b627c19049d4430ca951971ce2850732",
              "status": "untriaged"
          }, {
              "digest": "7eac1eebcc2250813ae69f2fbf3fdefd",
              "status": "untriaged"
          }, {
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive"
          }, {
              "digest": "081691a6a73bbaaf88a88926fd5e2228",
              "status": "positive"
          }, {
              "digest": "b05f062cdd0a52bc68a64c3cb3b0c808",
              "status": "untriaged"
          }, {
              "digest": "e7a14dd2617ddc77feeef0ec808a0e19",
              "status": "positive"
          }, {
              "digest": "5e6e7a68e4282bf03e9f50ab772850a1",
              "status": "untriaged"
          }],
          "total_digests": 8
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 2113,
              "pixelDiffPercent": 0.6603125,
              "maxRGBADiffs": [27, 63, 29, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.31041843,
                  "percent": 0.6603125,
                  "pixel": 2113
              },
              "digest": "6971d6faca77f9ddb05a4ae9243127c3",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["perf_alerts-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 27
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "7c0a393e57f14b5372ec1590b79bed0f",
      "test": "gold_details-page-sk",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-06-11T12:42:49.641532Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_details-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_details-page-sk,source_type=infra,",
              "data": [0, 0, 0, 0, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_details-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "7c0a393e57f14b5372ec1590b79bed0f",
              "status": "positive"
          }, {
              "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
              "status": "positive"
          }, {
              "digest": "29f31f703510c2091840b5cf2b032f56",
              "status": "untriaged"
          }, {
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 6496,
              "pixelDiffPercent": 0.71384615,
              "maxRGBADiffs": [38, 15, 56, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.31148747,
                  "percent": 0.71384615,
                  "pixel": 6496
              },
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_details-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 134
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "431a776ff5d646ab9c7476b05e321615",
      "test": "gold_digest-details-sk_no-refs",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_digest-details-sk_no-refs"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_digest-details-sk_no-refs,source_type=infra,",
              "data": [2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_digest-details-sk_no-refs",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "431a776ff5d646ab9c7476b05e321615",
              "status": "positive"
          }, {
              "digest": "0093a94fb96a18eb7f9bc0f33350d55f",
              "status": "untriaged"
          }, {
              "digest": "97fffffb5cf511b35a267dbe1696fd40",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 17757,
              "pixelDiffPercent": 4.743675,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.33581626,
                  "percent": 4.743675,
                  "pixel": 17757
              },
              "digest": "97fffffb5cf511b35a267dbe1696fd40",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_digest-details-sk_no-refs"],
                  "source_type": ["infra"]
              },
              "n": 138
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "97fffffb5cf511b35a267dbe1696fd40",
      "test": "gold_digest-details-sk_no-refs",
      "status": "positive",
      "triage_history": [{
          "user": "kjlubick@google.com",
          "ts": "2020-05-27T19:45:01.147062Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_digest-details-sk_no-refs"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_digest-details-sk_no-refs,source_type=infra,",
              "data": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_digest-details-sk_no-refs",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "97fffffb5cf511b35a267dbe1696fd40",
              "status": "positive"
          }, {
              "digest": "431a776ff5d646ab9c7476b05e321615",
              "status": "positive"
          }, {
              "digest": "0093a94fb96a18eb7f9bc0f33350d55f",
              "status": "untriaged"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 17757,
              "pixelDiffPercent": 4.743675,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.33581626,
                  "percent": 4.743675,
                  "pixel": 17757
              },
              "digest": "431a776ff5d646ab9c7476b05e321615",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_digest-details-sk_no-refs"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "29f31f703510c2091840b5cf2b032f56",
      "test": "gold_details-page-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_details-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_details-page-sk,source_type=infra,",
              "data": [3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_details-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "29f31f703510c2091840b5cf2b032f56",
              "status": "untriaged"
          }, {
              "digest": "971fe90fa07ebc2c7d0c1a109a0f697c",
              "status": "positive"
          }, {
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive"
          }, {
              "digest": "7c0a393e57f14b5372ec1590b79bed0f",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30348,
              "pixelDiffPercent": 3.334945,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.33654222,
                  "percent": 3.334945,
                  "pixel": 30348
              },
              "digest": "e49c92a2cff48531810cc5e863fad0ee",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_details-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 134
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "693d37373b6c349bcc8eb042b8b605fe",
      "test": "gold_diff-page-sk",
      "status": "untriaged",
      "triage_history": null,
      "paramset": {
          "ext": ["png"],
          "name": ["gold_diff-page-sk"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_diff-page-sk,source_type=infra,",
              "data": [2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_diff-page-sk",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "693d37373b6c349bcc8eb042b8b605fe",
              "status": "untriaged"
          }, {
              "digest": "a16db3e2e228c78a0833da3e2939ae4d",
              "status": "positive"
          }, {
              "digest": "955cc67da667b7e93685f8bd70b6d0fa",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 30348,
              "pixelDiffPercent": 3.334945,
              "maxRGBADiffs": [10, 10, 10, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.33654222,
                  "percent": 3.334945,
                  "pixel": 30348
              },
              "digest": "955cc67da667b7e93685f8bd70b6d0fa",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_diff-page-sk"],
                  "source_type": ["infra"]
              },
              "n": 138
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "1ef7f0dd6f83c7ee916dad8596aca631",
      "test": "gold_trace-filter-sk_nonempty",
      "status": "positive",
      "triage_history": [{
          "user": "lovisolo@google.com",
          "ts": "2020-06-30T17:38:42.778878Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_trace-filter-sk_nonempty"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_trace-filter-sk_nonempty,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_trace-filter-sk_nonempty",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "1ef7f0dd6f83c7ee916dad8596aca631",
              "status": "positive"
          }, {
              "digest": "ebcb48f48fffa6f584b9b939d6bcafd0",
              "status": "positive"
          }, {
              "digest": "ffadc0cf71439e9d9e5004f0f51a3053",
              "status": "untriaged"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 24449,
              "pixelDiffPercent": 5.0935416,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.3479799,
                  "percent": 5.0935416,
                  "pixel": 24449
              },
              "digest": "ebcb48f48fffa6f584b9b939d6bcafd0",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_trace-filter-sk_nonempty"],
                  "source_type": ["infra"]
              },
              "n": 56
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ebcb48f48fffa6f584b9b939d6bcafd0",
      "test": "gold_trace-filter-sk_nonempty",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_trace-filter-sk_nonempty"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_trace-filter-sk_nonempty,source_type=infra,",
              "data": [-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              "params": {
                  "ext": "png",
                  "name": "gold_trace-filter-sk_nonempty",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "ebcb48f48fffa6f584b9b939d6bcafd0",
              "status": "positive"
          }, {
              "digest": "ffadc0cf71439e9d9e5004f0f51a3053",
              "status": "untriaged"
          }, {
              "digest": "1ef7f0dd6f83c7ee916dad8596aca631",
              "status": "positive"
          }],
          "total_digests": 3
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 24449,
              "pixelDiffPercent": 5.0935416,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.3479799,
                  "percent": 5.0935416,
                  "pixel": 24449
              },
              "digest": "1ef7f0dd6f83c7ee916dad8596aca631",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_trace-filter-sk_nonempty"],
                  "source_type": ["infra"]
              },
              "n": 79
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "d8eddcc865ec838f247dae75914701f3",
      "test": "gold_query-dialog-sk_no-selection",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-07T23:55:27.190198Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_query-dialog-sk_no-selection"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_query-dialog-sk_no-selection,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_query-dialog-sk_no-selection",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "d8eddcc865ec838f247dae75914701f3",
              "status": "positive"
          }, {
              "digest": "052707cda34dbede1be0a2d204213c40",
              "status": "positive"
          }, {
              "digest": "ecd2352b87121127698d83c78e761bc6",
              "status": "positive"
          }, {
              "digest": "229ee20de719ecc69f43048c9da6abd5",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 25392,
              "pixelDiffPercent": 5.29,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.35462722,
                  "percent": 5.29,
                  "pixel": 25392
              },
              "digest": "ecd2352b87121127698d83c78e761bc6",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_query-dialog-sk_no-selection"],
                  "source_type": ["infra"]
              },
              "n": 19
          }
      },
      "closestRef": "pos"
  }, {
      "digest": "ecd2352b87121127698d83c78e761bc6",
      "test": "gold_query-dialog-sk_no-selection",
      "status": "positive",
      "triage_history": [{
          "user": "jcgregorio@google.com",
          "ts": "2020-07-09T17:15:56.999713Z"
      }],
      "paramset": {
          "ext": ["png"],
          "name": ["gold_query-dialog-sk_no-selection"],
          "source_type": ["infra"]
      },
      "traces": {
          "tileSize": 200,
          "traces": [{
              "label": ",name=gold_query-dialog-sk_no-selection,source_type=infra,",
              "data": [3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
              "params": {
                  "ext": "png",
                  "name": "gold_query-dialog-sk_no-selection",
                  "source_type": "infra"
              },
              "comment_indices": null
          }],
          "digests": [{
              "digest": "ecd2352b87121127698d83c78e761bc6",
              "status": "positive"
          }, {
              "digest": "052707cda34dbede1be0a2d204213c40",
              "status": "positive"
          }, {
              "digest": "d8eddcc865ec838f247dae75914701f3",
              "status": "positive"
          }, {
              "digest": "229ee20de719ecc69f43048c9da6abd5",
              "status": "positive"
          }],
          "total_digests": 4
      },
      "refDiffs": {
          "neg": null,
          "pos": {
              "numDiffPixels": 25392,
              "pixelDiffPercent": 5.29,
              "maxRGBADiffs": [7, 7, 7, 0],
              "dimDiffer": false,
              "diffs": {
                  "combined": 0.35462722,
                  "percent": 5.29,
                  "pixel": 25392
              },
              "digest": "d8eddcc865ec838f247dae75914701f3",
              "status": "positive",
              "paramset": {
                  "ext": ["png"],
                  "name": ["gold_query-dialog-sk_no-selection"],
                  "source_type": ["infra"]
              },
              "n": 21
          }
      },
      "closestRef": "pos"
  }],
  "offset": 0,
  "size": 105,
  "commits": [{
      "commit_time": 1592422850,
      "hash": "915a4938104e09e50b0f148220436ee9dfe3148e",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Port list-page-sk to lit-html",
      "is_cl": false
  }, {
      "commit_time": 1592422850,
      "hash": "f52c8f23cc673f13c5880da3c109ea4a5aed8cb3",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] use new by-list page",
      "is_cl": false
  }, {
      "commit_time": 1592422850,
      "hash": "f0678718512e1e4ad8f9ae842964fa2568a7e315",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] delete old list page",
      "is_cl": false
  }, {
      "commit_time": 1592423578,
      "hash": "fc002ae64a91fff06f26480a14ea93d34e6393e0",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Fix links on triagelog page",
      "is_cl": false
  }, {
      "commit_time": 1592425413,
      "hash": "7c98a621f9ce0ef2a5bece266419c56e8a3d7970",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Tweak trace dots color palette",
      "is_cl": false
  }, {
      "commit_time": 1592425413,
      "hash": "892675778a25b7e15468028c10834637f84d8b7d",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Re-assign digest colors, accounting for most used.",
      "is_cl": false
  }, {
      "commit_time": 1592426938,
      "hash": "9684e478f6fb4b41de1631ffda1cbfa16c5da523",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Link to source code with full blamelist.",
      "is_cl": false
  }, {
      "commit_time": 1592426948,
      "hash": "a73fdcf1a4df261928edf78df87dfcad5653b96e",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add notes to debug firestore usage",
      "is_cl": false
  }, {
      "commit_time": 1592445320,
      "hash": "08368432736162e44e9e1f96140991faf1c69024",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix demo pages.",
      "is_cl": false
  }, {
      "commit_time": 1592453179,
      "hash": "bff785024d44423e41697bbe5e63fe67a4b24565",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Convert day-range-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1592479586,
      "hash": "f25e7a6613dc31255cc99187004a2a2ce9bb7d0c",
      "author": "Ryan Harrison (rharrison@chromium.org)",
      "message": "Update child branch for shaderc",
      "is_cl": false
  }, {
      "commit_time": 1592481416,
      "hash": "a80ad7d70f66f5cffb195bbc5249b76b5a1dce94",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Obtain CIPD package details and bugs from tags",
      "is_cl": false
  }, {
      "commit_time": 1592494869,
      "hash": "1838cb950ea16f1396c1d5dcbdf846c5ea1529db",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[infra] Add file and function name to firestore metrics.",
      "is_cl": false
  }, {
      "commit_time": 1592494869,
      "hash": "88ed2a7fbdd8ae9c521770e30b81b007966fb507",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add more metrics to search and indexer.",
      "is_cl": false
  }, {
      "commit_time": 1592501876,
      "hash": "94f2cbe3721ead8f39deea1d493aad078e72287e",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port query-chooser-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1592503126,
      "hash": "7d165892fd9fb7565b27a39a2c6446f4a2072886",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port commit-detail-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1592505416,
      "hash": "16941f1a2bc61b3f276c404d93b4eb88a8a78631",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] Makefile: Update puppeteer_tests target.",
      "is_cl": false
  }, {
      "commit_time": 1592505943,
      "hash": "4e0bd800a0d6b2c10cd33230efb0f98940d19b39",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Update metrics and frontend",
      "is_cl": false
  }, {
      "commit_time": 1592506589,
      "hash": "d8c3ad848d26489f8129ab031a1e516682b18cd1",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] re-enable comments for chrome",
      "is_cl": false
  }, {
      "commit_time": 1592507867,
      "hash": "a55a0c2b25192900d658095d180b0aca63148a3d",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "Add GetReference and CreateReference to go/github",
      "is_cl": false
  }, {
      "commit_time": 1592509966,
      "hash": "2c4080564b80d0727cb4fc2b2fbf01d267e367d5",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] Fix Puppeteer tests.",
      "is_cl": false
  }, {
      "commit_time": 1592509986,
      "hash": "cfe951e940d914565acdaefc8bf12bdbcf1c37ab",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] Include Puppeteer tests in Infra-PerCommit-Puppeteer task.",
      "is_cl": false
  }, {
      "commit_time": 1592510888,
      "hash": "033768078c1dcf7f898e72bc356faeffb5f26035",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] search-controls-sk.html: Remove obsolete call to the Polymer-based query-dialog-sk's close() method.",
      "is_cl": false
  }, {
      "commit_time": 1592536294,
      "hash": "fcf6dfc729de968ce35f051a764f7763c8ee3b24",
      "author": "Weston Tracey (westont@google.com)",
      "message": "Make eventPromise listen to document, so error-sk is caught.",
      "is_cl": false
  }, {
      "commit_time": 1592570749,
      "hash": "a08250a3e8c23d020331bfa00bcd9d64800c3a8b",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add (somewhat) helpful help page",
      "is_cl": false
  }, {
      "commit_time": 1592574950,
      "hash": "3060cfae7069e850b7875f46a21d8f5a48a5f396",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[infra-sk] Add gold tests for query-sk.",
      "is_cl": false
  }, {
      "commit_time": 1592579790,
      "hash": "dc56391f171b283c4898a7076f7530f2c0e3ae54",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "Delete dead metrics code",
      "is_cl": false
  }, {
      "commit_time": 1592580020,
      "hash": "b18a31da1f061769ee8737491a71d307ae084466",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "Fix egde typo (again)",
      "is_cl": false
  }, {
      "commit_time": 1592589807,
      "hash": "5f614e46f5adb97185d25c97dc176e705079a751",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Don't error on CL not found",
      "is_cl": false
  }, {
      "commit_time": 1592616559,
      "hash": "f750590147996aeb901535dff9ff19e3f2285574",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "sort-sk to typescript.",
      "is_cl": false
  }, {
      "commit_time": 1592661916,
      "hash": "cab39d1442120ce5f5f1ae60c39685073158ffc4",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "Fix appname for some infra-sk tests.",
      "is_cl": false
  }, {
      "commit_time": 1592665760,
      "hash": "23b7fda631d54cdddad762a68671df6fcf6efd76",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "algo-select-ts",
      "is_cl": false
  }, {
      "commit_time": 1592717000,
      "hash": "9d47158ed15a3f70f1650f63eba61d665bc4a494",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update CIPD Packages",
      "is_cl": false
  }, {
      "commit_time": 1592743920,
      "hash": "6cf7e3a36a9fb9540f36f32895fd3bf6a18c77d7",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autorollers] Add supportsManualRoll to Skia-\u003eFlutter",
      "is_cl": false
  }, {
      "commit_time": 1592844302,
      "hash": "ec8409605ca8f5cf48ed463bd5c270637e0847f3",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add value-changed event to suggest-input-sk",
      "is_cl": false
  }, {
      "commit_time": 1592846032,
      "hash": "3632b9ef7dd885f962249c959983a32aa3f4ef76",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[infra-sk] Manually recompute textarea size on expand in expandable-textarea-sk. Necessary if value is set prior to making the element visible.",
      "is_cl": false
  }, {
      "commit_time": 1592848997,
      "hash": "144993a82a595aeffd73ec461e021bbd7c8a5ab3",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[task driver] Fix nil-dereference in display",
      "is_cl": false
  }, {
      "commit_time": 1592849157,
      "hash": "8a05f82240eeeef2fb4abc25a1ed7eb01efcb031",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] Port filter-dialog-sk to lit-html.",
      "is_cl": false
  }, {
      "commit_time": 1592849216,
      "hash": "7d3c4130eca10b626b24b52d9bec79bd2a3976f6",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add chrome service account to frontend",
      "is_cl": false
  }, {
      "commit_time": 1592849987,
      "hash": "56bef8bee90d5832fa5091896a8ba4f9e4444558",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Fix bare html",
      "is_cl": false
  }, {
      "commit_time": 1592850407,
      "hash": "8e9a9b9a5455ab6ee1ba8846acdfccd4507b3b36",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] Use the lit-html version of filter-dialog-sk.",
      "is_cl": false
  }, {
      "commit_time": 1592854507,
      "hash": "a5d185a27c433727d267196617ea85d671c0ab1c",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add chromium-perf-sk element.",
      "is_cl": false
  }, {
      "commit_time": 1592904723,
      "hash": "095957759778ccee8b00fe2a1cf27812390b04df",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] search-controls-sk: Initial skeleton code.",
      "is_cl": false
  }, {
      "commit_time": 1592914773,
      "hash": "26c69790a4f6114bc22578017a235444e0dbe9b2",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] No error for failure to retrieve sheriff",
      "is_cl": false
  }, {
      "commit_time": 1592923983,
      "hash": "48137d2ce6d2d0f3c6b8886d5a6abe84a836031d",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Add new bucket for Flutter instance.",
      "is_cl": false
  }, {
      "commit_time": 1592931683,
      "hash": "7ecf290a1fc8e4851d0caf7cd973e021b84929e1",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Remove skiaperf executable.",
      "is_cl": false
  }, {
      "commit_time": 1592938014,
      "hash": "9f9d4316defdfdd549cd26e2e8596d885b7b42e9",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autoroller] Cleanup github fork branches older than a week",
      "is_cl": false
  }, {
      "commit_time": 1592940083,
      "hash": "b195e89bbf497c505f3940a923ca9ea7cf8b2745",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Remove unused flag.",
      "is_cl": false
  }, {
      "commit_time": 1592941623,
      "hash": "236e1d7dae45212d69e453fcbb27191dfc082d42",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix cluster sub-command.",
      "is_cl": false
  }, {
      "commit_time": 1592941743,
      "hash": "02bd88e3cf6c7280448b0494aaa30328332c4574",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[demos] Add public service account script",
      "is_cl": false
  }, {
      "commit_time": 1592942573,
      "hash": "5825563c657ba4333a0b142cc816f17e7b7bb797",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[demos] Update skfe for demos",
      "is_cl": false
  }, {
      "commit_time": 1593002489,
      "hash": "9028592373a3ee5a295cc2f1208309a4d62e9f8b",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Update images",
      "is_cl": false
  }, {
      "commit_time": 1593010049,
      "hash": "cab905effaf1c8a4e95b09f2ce2141cfe2875d88",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Limit link color CSS to patch-sk",
      "is_cl": false
  }, {
      "commit_time": 1593010679,
      "hash": "c9ac1dc9e239db21ad6d42679b1fe51cbf9b064f",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Fix ct-scaffold chromium_perf link.",
      "is_cl": false
  }, {
      "commit_time": 1593026189,
      "hash": "9e1cf3de4df77b2b9a797f3462374c029ed8bf23",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add chromium_perf page.",
      "is_cl": false
  }, {
      "commit_time": 1593094640,
      "hash": "be64fcd3950350f08966b178951d8a159d2b93b2",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[query-sk] Fix regex queries.",
      "is_cl": false
  }, {
      "commit_time": 1593181061,
      "hash": "c23c2aa69fc1d19e2059ba0c60bdd8108a52281f",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Convert perf-scaffold-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1593181222,
      "hash": "32627f70fbc59fc2a9293e6a85bb8bf92de761c7",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Make puppeteer tests much faster and use less RAM.",
      "is_cl": false
  }, {
      "commit_time": 1593193671,
      "hash": "4d37bf5d78c7a20cd6f1e9e341640af010222c06",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Fix missing handler for Runs History in chromium-perf-sk",
      "is_cl": false
  }, {
      "commit_time": 1593195301,
      "hash": "0506beb1a6f910b9b30f9eacbc20b18e18d763c7",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] Extract trace-filter-sk component out of filter-dialog-sk.",
      "is_cl": false
  }, {
      "commit_time": 1593197161,
      "hash": "d7783ba31f7b41330e3d972c0195addd8fc890a0",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] corpus-selector-sk: Port to TypeScript and simplify.",
      "is_cl": false
  }, {
      "commit_time": 1593200691,
      "hash": "b9f63db4dbe86f23c8eeac97f220475f2f246391",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] search-controls-sk: Implement lit-html component.",
      "is_cl": false
  }, {
      "commit_time": 1593200701,
      "hash": "ad618eb5a4bbc52c22925b1252f469e07f054cac",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Convert commit-detail-panel-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1593204121,
      "hash": "1be3fbb1e03af485133d586369f1505f2d1fecf9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port commit-detail-picker-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1593204711,
      "hash": "c630087bf6b70e30a1dd291272e013e278749921",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix randomness in plot-simple-sk Gold images.",
      "is_cl": false
  }, {
      "commit_time": 1593206841,
      "hash": "b7b816d7e1694ff07a8a99f9c229510940f39845",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Experiment with tslint.",
      "is_cl": false
  }, {
      "commit_time": 1593228421,
      "hash": "60281e15926171777cc997ea02ab2cdd600edd5e",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] search-controls-sk: Use the new lit-html component everywhere.",
      "is_cl": false
  }, {
      "commit_time": 1593321783,
      "hash": "ccb305d05cd36cf81dfa8712daef092848a7c032",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update CIPD Packages",
      "is_cl": false
  }, {
      "commit_time": 1593442235,
      "hash": "026a27eb2582113e3377537bb08a8ac24c43387b",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add chromium-analysis-sk element.",
      "is_cl": false
  }, {
      "commit_time": 1593448162,
      "hash": "2be37a93d661b72ac1ad76779095b9e867c542a0",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Make goldpushk relative from golden subfolder",
      "is_cl": false
  }, {
      "commit_time": 1593448162,
      "hash": "ae98cd6c5590603a3e08b28d890c82772efdb546",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Port diffserver to using JSON config",
      "is_cl": false
  }, {
      "commit_time": 1593448162,
      "hash": "67ad34a54b6135241853eaeb84bc5a970db94814",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Refactor ingestion configs",
      "is_cl": false
  }, {
      "commit_time": 1593448162,
      "hash": "065737b1c16da25a970f5928e95537ab2bb728d1",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Port ingestion-bt to use JSON instead of flags+JSON.",
      "is_cl": false
  }, {
      "commit_time": 1593448162,
      "hash": "dba6b285e59ed0f23c2fd1661246097fd50a1f5c",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Port final service to JSON5 config. Delete old configs.",
      "is_cl": false
  }, {
      "commit_time": 1593450044,
      "hash": "5b4399509651557033f47216c82f6bfb45f8d1e9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Standup perf flutter-flutter instance.",
      "is_cl": false
  }, {
      "commit_time": 1593451773,
      "hash": "5f2682aa9683a86c5e0ce3b61862e02c3e46b8af",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Update to clean images using JSON config",
      "is_cl": false
  }, {
      "commit_time": 1593452815,
      "hash": "08f1164afdb2e4aa803a4610e6e8398a83587abf",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix for plot-simple-sk.",
      "is_cl": false
  }, {
      "commit_time": 1593452985,
      "hash": "6809c193328f1f6cc23920c35e4aefc717c971a1",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] triage-status fixes.",
      "is_cl": false
  }, {
      "commit_time": 1593452995,
      "hash": "f281e74bfcead8385664b08e722dfd267ccd31b9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Move flutter-perf.skia.org to flutter-engine-perf.skia.org",
      "is_cl": false
  }, {
      "commit_time": 1593453835,
      "hash": "fb0713f028085c39e77e2591942ed97e463f5c0e",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Make indexer use timestamp on tryjob results.",
      "is_cl": false
  }, {
      "commit_time": 1593453835,
      "hash": "4c0fef1593677728bd2394ee148ab58c38bd445a",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Use time in tryjob results",
      "is_cl": false
  }, {
      "commit_time": 1593456285,
      "hash": "6448e5e770bcc94c28ee6ea0af2839347af8b6cd",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Fix config and push new ingestion",
      "is_cl": false
  }, {
      "commit_time": 1593529394,
      "hash": "9fa0af1e2e8b126c7f9cf57472c11c7444d87b9d",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add anti-affinity to improve uptime",
      "is_cl": false
  }, {
      "commit_time": 1593530721,
      "hash": "6d1199459660b810dda7fb9a1748281989c54e0f",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Redirect to flutter at the new address.",
      "is_cl": false
  }, {
      "commit_time": 1593533965,
      "hash": "7007683e3627a013d42e04a342800609ba164f45",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix selectable on commit-detail-panel-sk instances.",
      "is_cl": false
  }, {
      "commit_time": 1593539085,
      "hash": "28484fe00d2b3ab501153966fa872ff3d9d154b8",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] calendar-sk",
      "is_cl": false
  }, {
      "commit_time": 1593541135,
      "hash": "5bf4085d29f17e40378ce0b39cd8883c9d94ba17",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[webtools] Use \"npm ci\" instead of \"npm install\"",
      "is_cl": false
  }, {
      "commit_time": 1593580945,
      "hash": "d944fa9523637794c63a853d40aff04a3cc64b0a",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update Go Deps",
      "is_cl": false
  }, {
      "commit_time": 1593617328,
      "hash": "cda3473da077e7372bc350e002ea96abf9525706",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Add perf-tool backup command.",
      "is_cl": false
  }, {
      "commit_time": 1593621018,
      "hash": "27fa3d05a90511fdba7817b229faf167269694cd",
      "author": "Jeff Yoon (jeffyoon@google.com)",
      "message": "Update PGO Autorollers with Beta and Contact Info",
      "is_cl": false
  }, {
      "commit_time": 1593624186,
      "hash": "7de50cb43bff0f5d932d7eccba9bb01434d6c53f",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[CT] Fix bug where run_on_gce=false for Windows",
      "is_cl": false
  }, {
      "commit_time": 1593625457,
      "hash": "f9847d7a3351d4a85b82bff949de8e9e8cb830c3",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add chromium_analysis page",
      "is_cl": false
  }, {
      "commit_time": 1593631566,
      "hash": "a910315e976e556253015f23c6f4e527a049cbf5",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[debugger_assets] Fix broken build target",
      "is_cl": false
  }, {
      "commit_time": 1593632486,
      "hash": "3f0ed381eedc05be8fd8468385791d42ac744cb9",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Re-render ct-scaffold once we have queue length.",
      "is_cl": false
  }, {
      "commit_time": 1593632600,
      "hash": "a4f02654ea2ce1d692c60fa0069f80fb8fac5449",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[sort-sk] Support use in table and expose sort()",
      "is_cl": false
  }, {
      "commit_time": 1593632600,
      "hash": "18cbfe7552632f5927dc6b4ba065ead9815fb8be",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Use sort-sk on list-page-sk",
      "is_cl": false
  }, {
      "commit_time": 1593632600,
      "hash": "0e2b9fff86153d611df6af451d67f81f5fdb1750",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add links to by-list for subset of digests",
      "is_cl": false
  }, {
      "commit_time": 1593634876,
      "hash": "4cbfff6d6e4d23a552981218a64fd2fa0c657eef",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] calendar-input-sk.",
      "is_cl": false
  }, {
      "commit_time": 1593639986,
      "hash": "cfad866369daddc5b9f208ed8ff53f24f1155c91",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Move day-range-sk from input type=date to calendar-input-sk.",
      "is_cl": false
  }, {
      "commit_time": 1593646037,
      "hash": "f65049e8d1995b59bfebe2c39f951cc945ffff4f",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[CT] Remove SK_WHITELIST_SERIALIZED_TYPEFACES from CT",
      "is_cl": false
  }, {
      "commit_time": 1593709667,
      "hash": "9ec95c07091380e3991e4b9d9f92f4c163a6e4df",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port domain-picker-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1593832132,
      "hash": "fa7211d2ad9becb0791d424f8429ce7144635a18",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port clsuter-summary2-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1593926413,
      "hash": "5b8591b50b2475f259fc2c73692a8ce195e2cfdf",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update CIPD Packages",
      "is_cl": false
  }, {
      "commit_time": 1593959214,
      "hash": "e317f954b75893979a875f50ba32e2e4f099b367",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port alert-config-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594012779,
      "hash": "d75a88d1bb625bf39335d97cbd464b9e29f5624b",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update Go Deps",
      "is_cl": false
  }, {
      "commit_time": 1594040060,
      "hash": "7f69500c35288c9ea60904f5c4d7091777c72bb1",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add chrome-public instance",
      "is_cl": false
  }, {
      "commit_time": 1594056570,
      "hash": "98b77200d1f7531ba5a15e86b4eec09b6e817872",
      "author": "Tiancong Wang (tcwang@google.com)",
      "message": "[autoroll] Manually update cros afdo rollers",
      "is_cl": false
  }, {
      "commit_time": 1594057104,
      "hash": "1324d5114c84dde45cb249edd3a96a156cc2dbcc",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add metrics-analysis-sk",
      "is_cl": false
  }, {
      "commit_time": 1594057500,
      "hash": "8860a9911ef243baa630150af326bf862b25c92c",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port triage-page-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594057737,
      "hash": "fd65c33dd11ac0b3232e97656ff89c2cf3037fc8",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[envoy] Remove some deprecated types/warnings",
      "is_cl": false
  }, {
      "commit_time": 1594057900,
      "hash": "fe7626a48f6db9493d6c56e37039eaf6e0f4e6f0",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add now label to force redeployment",
      "is_cl": false
  }, {
      "commit_time": 1594057920,
      "hash": "b95c52ff254f1209bb10fe53d7c5d8be40229672",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Fix links for positive/negatives",
      "is_cl": false
  }, {
      "commit_time": 1594060343,
      "hash": "3efa59a7bdc4c427ab39cfb0c37862a8b449abaf",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Make flutter-flutter-perf.skia.org public.",
      "is_cl": false
  }, {
      "commit_time": 1594069700,
      "hash": "5912bed81b5801c952dc4485e1a9538ea3d0a9e8",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Convert alert-page-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594069851,
      "hash": "88e733d409bf8eb45c42794d4d31e788e0148c41",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add metrics_analysis page",
      "is_cl": false
  }, {
      "commit_time": 1594099551,
      "hash": "0c968ed09a69027f69054b7c54547552de23d2ad",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update Go Deps",
      "is_cl": false
  }, {
      "commit_time": 1594132524,
      "hash": "29b6ac5b5dda6eef17412f508c0e37b6683d25b9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Follup-up fixes for triage-page-sk.",
      "is_cl": false
  }, {
      "commit_time": 1594142854,
      "hash": "a1ef758371f48f7b5c762f7cbf701312a985b2d1",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] New contacts/reviewers for depot tools -\u003e chromium roller",
      "is_cl": false
  }, {
      "commit_time": 1594143264,
      "hash": "e0050ef02d8b60ffccab9761c2b562eee2bd2328",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autorollers] Add support for canaries",
      "is_cl": false
  }, {
      "commit_time": 1594144804,
      "hash": "e3fd453fe6eabd0406f3dc81d52125f3ea260d35",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autoroller] Turn on manual rolls for dart-\u003eflutter",
      "is_cl": false
  }, {
      "commit_time": 1594148746,
      "hash": "4fc63f24215cbab97f4c68efd91b2b9922858ac2",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Follow-up fix for alert-page-sk.",
      "is_cl": false
  }, {
      "commit_time": 1594149354,
      "hash": "cab388a0127c610756b23658bf6db3032066a43a",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Pert explore-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594155754,
      "hash": "62a5a39a9efc2910c12b5b234b4c362892423891",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port cluster-page-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594156854,
      "hash": "f32f3d0efed3ec22b22870c4e5f91e82afb72faa",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[query-sk] Fix styling of the filter input.",
      "is_cl": false
  }, {
      "commit_time": 1594175214,
      "hash": "73d6a9cf2efd672628bba8905ae3a06c06308b36",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Port cluster-lastn-page-sk to TypeScript.",
      "is_cl": false
  }, {
      "commit_time": 1594175334,
      "hash": "b5d0551908826a934caf03ed92e55d3be885a177",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Add theme-chooser-sk to perf-scaffold-sk.",
      "is_cl": false
  }, {
      "commit_time": 1594210298,
      "hash": "7b220d29fd618f448f811782fe9e65b43d0408b2",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add chromium-build-selector-sk",
      "is_cl": false
  }, {
      "commit_time": 1594210307,
      "hash": "41abdffdd2158690f0a2ffcf5c681306d81db9a4",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add capture-skps-sk element.",
      "is_cl": false
  }, {
      "commit_time": 1594210316,
      "hash": "83a501cfa80b9a855e50db7de50099b7002018d9",
      "author": "Weston Tracey (westont@google.com)",
      "message": "[ct] Add capture_skps page.",
      "is_cl": false
  }, {
      "commit_time": 1594213855,
      "hash": "074cf9058409fa4b94192477bb12505497cc9fdb",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[Autoroller] Update freetype-chromium merge paths.",
      "is_cl": false
  }, {
      "commit_time": 1594216065,
      "hash": "fcce5bb3db4b0d55cee6a28a0645209d259525d7",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autoroller] Fix Dockerfile",
      "is_cl": false
  }, {
      "commit_time": 1594223425,
      "hash": "363e801179b8ca8b0857cdf101901c1ceea7ee24",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autoroll] Add Get method to manual/db.go",
      "is_cl": false
  }, {
      "commit_time": 1594224365,
      "hash": "39bbe4780f836930f07c9ca44aeefb43a5ef279f",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autoroller] Add ability to skip emails",
      "is_cl": false
  }, {
      "commit_time": 1594227415,
      "hash": "1b9f11dac68a05704a0e8e81a24979ee2e35b3f6",
      "author": "Jeff Yoon (jeffyoon@google.com)",
      "message": "Adding additional sheriff to PGO autorollers",
      "is_cl": false
  }, {
      "commit_time": 1594228285,
      "hash": "32030e34124d0d756b3fb08c5448e80627e7228a",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] Add PageObjectElement class.",
      "is_cl": false
  }, {
      "commit_time": 1594228545,
      "hash": "03f9117e14567b4214357ad8888c309c04b80a3c",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[infra-sk] Remove double scrollbar",
      "is_cl": false
  }, {
      "commit_time": 1594238455,
      "hash": "59f3bd6a902847a9e8d7966cfb4c8fab035c7196",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix cluster-page-sk",
      "is_cl": false
  }, {
      "commit_time": 1594240555,
      "hash": "a03af3aaa9a46992dbbddef7a7203e597c681513",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Add a local themes.scss.",
      "is_cl": false
  }, {
      "commit_time": 1594247945,
      "hash": "6cff6cc521b6f0ffb61c9c9c3615c59e0b3d9239",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[infra-sk] Fix banding on paramset-sk.",
      "is_cl": false
  }, {
      "commit_time": 1594251545,
      "hash": "63f1e762b9d60dde6533a02f14980177c007ffdc",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] paramset-sk: Add a page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594251885,
      "hash": "5cc491ae48e022b1eae9a1ed450933653e90c6d9",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] query-values-sk: Add a page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594251925,
      "hash": "8b426883d0fe3aa21ceeabbe076cc651f78e2ff9",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[infra-sk] query-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594298616,
      "hash": "fd71f156e6c1b554d6952b7124c4917970af74e5",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[particles] Expose version info about canvaskit build",
      "is_cl": false
  }, {
      "commit_time": 1594307189,
      "hash": "6ffacc77fc578b66c4f180d40928bb476d0dae3f",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix perf-scaffold and theme CSS.",
      "is_cl": false
  }, {
      "commit_time": 1594313319,
      "hash": "66f797e32f53640ddaa69c985d31062fac7c18b9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf][golden] Upgrade elements-sk to 3.3.4.",
      "is_cl": false
  }, {
      "commit_time": 1594315079,
      "hash": "d743a7d95130b863da3ec485a5dbbc876cff64bc",
      "author": "Brian Osman (brianosman@google.com)",
      "message": "Update default effect on particles.skia.org",
      "is_cl": false
  }, {
      "commit_time": 1594321229,
      "hash": "f5b48d60d9dcb3e58e0001b026ef380cb4a66a99",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] query-dialog-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594322219,
      "hash": "4ad3587da0164c4ddc2d6c9cec2e0a46e39d81f4",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] trace-filter-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594322790,
      "hash": "83cd41fd102bcdb1915596dc05c2b484b805d63c",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add cluster-digests-sk element.",
      "is_cl": false
  }, {
      "commit_time": 1594322790,
      "hash": "28157d9c4a77178f8a348868ea8f5c24e70b0f0c",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add helpers for dealing with URL params to search-controls-sk",
      "is_cl": false
  }, {
      "commit_time": 1594323299,
      "hash": "b44c941764aebd6ccb47122f47f868e6e4f3955f",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] filter-dialog-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594325479,
      "hash": "2a251850405ca966dda93bc1d481396e889ae38b",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] corpus-selector-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594325577,
      "hash": "ed267d27ef91d51d66032783cf5e1f295e5967e2",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Style the scrollbars.",
      "is_cl": false
  }, {
      "commit_time": 1594329579,
      "hash": "9acc96b0753c64413a742cae128f08582ace6968",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Don't display LSE where it doesn't apply.",
      "is_cl": false
  }, {
      "commit_time": 1594387930,
      "hash": "1edb3316298fd299eb2a74402557dc3db413cda8",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autorollers] Do not add reviews to repo upload if no emails specified",
      "is_cl": false
  }, {
      "commit_time": 1594392714,
      "hash": "a7fd6c7ec66b93c5ea0c79f6ba5c401e317a95ce",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Follow-up for scrollbar styling.",
      "is_cl": false
  }, {
      "commit_time": 1594392872,
      "hash": "829d9acd9d58e7ced801e208b5dc9342a1e47706",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Add click handlers to cluster-digests-sk",
      "is_cl": false
  }, {
      "commit_time": 1594394000,
      "hash": "11610204ae961e4722262ef17afee82926f8571f",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "NewDataFrameIterator",
      "is_cl": false
  }, {
      "commit_time": 1594394300,
      "hash": "e24d7a62c06918bd7a395d65a027542423d73174",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Make cluster-page and lastn-page puppeteer tests deterministic.",
      "is_cl": false
  }, {
      "commit_time": 1594398630,
      "hash": "f165a841a32d0d100be6a7053265b6fdc4540a67",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] cluster-summary2-sk show commit details for xbar.",
      "is_cl": false
  }, {
      "commit_time": 1594408381,
      "hash": "574bd22f7107a7c3b80255210463286a66427d8e",
      "author": "Sebastien Marchand (sebmarchand@chromium.org)",
      "message": "Update sebmarchand's email address for the PGO profile rolls",
      "is_cl": false
  }, {
      "commit_time": 1594409170,
      "hash": "b26acb953c927b50e6283b089cf9dcbf45a8793c",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Inclusive language",
      "is_cl": false
  }, {
      "commit_time": 1594415761,
      "hash": "78696c00978453d3b835a6682c82bf02a62de575",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] LSE calc for OriginalStep is wrong.",
      "is_cl": false
  }, {
      "commit_time": 1594416021,
      "hash": "a615c7531284211220d675314c4c3cf61f61052c",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[infra-sk] Move the fast filter on query-sk.",
      "is_cl": false
  }, {
      "commit_time": 1594476795,
      "hash": "31d53aac2f8f03a88993afc081282b75a6c94004",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Have ./modules/json/index.ts have proper dependencies.",
      "is_cl": false
  }, {
      "commit_time": 1594531254,
      "hash": "d749d74a95efc781ce0bb017d2a4b127621dff94",
      "author": "skia-recreate-skps (skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com)",
      "message": "Update CIPD Packages",
      "is_cl": false
  }, {
      "commit_time": 1594560830,
      "hash": "d3550732bff319d9b294b0081fc5cb9cfea81a62",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix explore query and skp markers.",
      "is_cl": false
  }, {
      "commit_time": 1594561923,
      "hash": "4a603f29e184ed0328eab94cbadf3f0ff967eb23",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Fix sidebar",
      "is_cl": false
  }, {
      "commit_time": 1594577125,
      "hash": "c55e15d570b1572a4384c4d537b1bf58123822d7",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] search-controls-sk: Add page object and use it in tests.",
      "is_cl": false
  }, {
      "commit_time": 1594578745,
      "hash": "2dba6c85dedc95ec7f51ee28191fd57d8426878b",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] Deflake gold_query-dialog-sk_multiple-values-selected.",
      "is_cl": false
  }, {
      "commit_time": 1594641309,
      "hash": "d2cf0ee78159f379d506c12ecb13179b767205fa",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Remove Zoom Range button.",
      "is_cl": false
  }, {
      "commit_time": 1594641350,
      "hash": "2a155d19f97622052ad6b54e93178739a26cc99d",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Style fixes for inputs and buttons.",
      "is_cl": false
  }, {
      "commit_time": 1594641376,
      "hash": "36d176536e1725bd6d138ee977096bc240a1c4fa",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Actually load the Roboto font.",
      "is_cl": false
  }, {
      "commit_time": 1594641517,
      "hash": "e9512257b55ada3101ae3b4a165cfb3ba94c8f5b",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Disable details tab on explore when disabled.",
      "is_cl": false
  }, {
      "commit_time": 1594645203,
      "hash": "43b728cf27f09e0fa285a8c0214050802e7644d5",
      "author": "Eric Boren (borenet@google.com)",
      "message": "Fix some non-inclusive language",
      "is_cl": false
  }, {
      "commit_time": 1594647333,
      "hash": "2630055acc2f94f5002611c6262604b483b054df",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autorollers] Do not \"wait\" for tree-status checks during dry runs",
      "is_cl": false
  }, {
      "commit_time": 1594652733,
      "hash": "d4a9962e2adc2d0aa9c2324414f84721fc8b6da4",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] More consolidation on things that are commit numbers.",
      "is_cl": false
  }, {
      "commit_time": 1594654373,
      "hash": "5be44e3348441b2255ad692c23b7ac4e8cb94be0",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[am] Fix double scrollbars.",
      "is_cl": false
  }, {
      "commit_time": 1594658786,
      "hash": "8c43148a19ca347041bc159ae775f7890dcce4e9",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Tighten up buttons.",
      "is_cl": false
  }, {
      "commit_time": 1594661722,
      "hash": "bd7b96721c4294fceb165fbc76ef249204afa71d",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Tighten up buttons.",
      "is_cl": false
  }, {
      "commit_time": 1594664993,
      "hash": "1e873b7fb3987130f65036d6d725646250d89f39",
      "author": "Tiancong Wang (tcwang@google.com)",
      "message": "[autoroll] Manually update cros orderfile rollers",
      "is_cl": false
  }, {
      "commit_time": 1594666443,
      "hash": "88ffb83541286cef05309cd97de2ca1f8c1a8202",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Add URL to fake commit message data",
      "is_cl": false
  }, {
      "commit_time": 1594667253,
      "hash": "01a7d83dcf27c85965239edd20e43ff32a7203f5",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "Update go2ts to v.1.3.2.",
      "is_cl": false
  }, {
      "commit_time": 1594667273,
      "hash": "d1daff06b038e9f0f37b2b0c6e62491a8e7a1c21",
      "author": "Leandro Lovisolo (lovisolo@google.com)",
      "message": "[gold] Add code generator for TypeScript RPC types.",
      "is_cl": false
  }, {
      "commit_time": 1594735088,
      "hash": "757374e852412ae69194669c5d32a6bfc59486c6",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[perf] Hide buttons that apply to highlighted traces.",
      "is_cl": false
  }, {
      "commit_time": 1594749882,
      "hash": "6227f9af3ae06637a08db370d6725ae4ae27b11f",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Use proper templates for CL messages",
      "is_cl": false
  }, {
      "commit_time": 1594752126,
      "hash": "ddec7447f21cafd641614e65ebe3ecb88749ef00",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[infra-sk] Move setQueryString to test_util",
      "is_cl": false
  }, {
      "commit_time": 1594752126,
      "hash": "cfc32e173bb45a0d30c22e2c3e7066073aa40288",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Initial cluster-page-sk",
      "is_cl": false
  }, {
      "commit_time": 1594752654,
      "hash": "553c6232549653161692528627dc875883987590",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Longer display revision for CIPD packages",
      "is_cl": false
  }, {
      "commit_time": 1594752784,
      "hash": "1cdb9a55b55a330dbaaed7c47fd6c913a0527510",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Migrate to Lit-HTML and TypeScript",
      "is_cl": false
  }, {
      "commit_time": 1594753677,
      "hash": "70979f673e3693fa15fe7fd427e1f9f6943318fc",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[machine] Start recording when bot_config started running.",
      "is_cl": false
  }, {
      "commit_time": 1594757287,
      "hash": "15957372ea630f7f2467c1db89a775323478dc12",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "Revert \"[named-fiddles] Remove named-fiddles.\"",
      "is_cl": false
  }, {
      "commit_time": 1594757497,
      "hash": "7cc820a90366ff31415813c0881503325b9ed553",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Remove need for goldctl's service account to have read access.",
      "is_cl": false
  }, {
      "commit_time": 1594758085,
      "hash": "b59652b6cfb3ce26053048486d0f0b922fe067f6",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[named-fiddles] Fix release and alert message.",
      "is_cl": false
  }, {
      "commit_time": 1594758914,
      "hash": "da6ab914994ad85537fe236a15e2762f89d497aa",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Update with clean image",
      "is_cl": false
  }, {
      "commit_time": 1594760687,
      "hash": "bc871511ec55f419c429fbee4edebddb7f622bf3",
      "author": "Ravi Mistry (rmistry@google.com)",
      "message": "[Autorollers] When rev is a patch ref use gclient's patch-ref args",
      "is_cl": false
  }, {
      "commit_time": 1594762977,
      "hash": "fc9bf78ccfe261f03094c4ba5558a160b80e499b",
      "author": "Kevin Lubick (kjlubick@google.com)",
      "message": "[gold] Remove unused executables",
      "is_cl": false
  }, {
      "commit_time": 1594764007,
      "hash": "d9a61cf86f56e12924fef7b5aeaa63fb3963f781",
      "author": "Joe Gregorio (jcgregorio@google.com)",
      "message": "[machine] Schedule pods for restart if they get too old.",
      "is_cl": false
  }, {
      "commit_time": 1594812819,
      "hash": "251e0be180076aac9c9ab2964cea6d88f78023e2",
      "author": "Eric Boren (borenet@google.com)",
      "message": "Fix Infra-PerCommit-Build",
      "is_cl": false
  }, {
      "commit_time": 1594817699,
      "hash": "3e53cd963f319a3e3e293bd091e83149eab703f6",
      "author": "Eric Boren (borenet@google.com)",
      "message": "[autoroll] Fixes",
      "is_cl": false
  }],
  "trace_comments": null,
  "bulk_triage_data": {
      "gold_details-page-sk": {
          "29f31f703510c2091840b5cf2b032f56": "positive",
          "7c0a393e57f14b5372ec1590b79bed0f": "positive",
          "971fe90fa07ebc2c7d0c1a109a0f697c": "positive",
          "e49c92a2cff48531810cc5e863fad0ee": "positive"
      },
      "gold_diff-page-sk": {
          "693d37373b6c349bcc8eb042b8b605fe": "positive",
          "955cc67da667b7e93685f8bd70b6d0fa": "positive",
          "a16db3e2e228c78a0833da3e2939ae4d": "positive"
      },
      "gold_digest-details-sk": {
          "30618d40e17fc4edf6df6bc6bbb17b5f": "positive",
          "6e9090b378162a88d8815046562ed1e9": "positive",
          "e202ae8d070a3a0ce9f5e1c30bd254ba": "positive",
          "f9d2c4bbc5f8df84963f5fe65a0c522d": "positive"
      },
      "gold_digest-details-sk_changelist-id": {
          "5598719c5f68bd6b970e141e39209465": "positive",
          "71782303a38cae9eea9d2a3ae517b774": "positive",
          "81351eb9c5b2bb38106a90acbe863185": "positive",
          "ba249337c6ea1bee69e5245bdd62a140": "positive"
      },
      "gold_digest-details-sk_negative-only": {
          "92137d4b89a7be781008094acc232308": "positive",
          "9931070715675310cabe7e26561b1896": "positive",
          "c0a25c6b2262c8c3346181f427327b84": "positive",
          "c2e28c0c694d45aa3684c5f16babeef0": "positive"
      },
      "gold_digest-details-sk_no-params": {
          "46eec311a09cc32f79799009260a89cc": "positive",
          "a8f92ab75a783239ca7ac09befdec45e": "positive",
          "fcfd6c87b5c58298c7b6f1ee10dec701": "positive"
      },
      "gold_digest-details-sk_no-refs": {
          "0093a94fb96a18eb7f9bc0f33350d55f": "positive",
          "431a776ff5d646ab9c7476b05e321615": "positive",
          "97fffffb5cf511b35a267dbe1696fd40": "positive"
      },
      "gold_digest-details-sk_no-traces": {
          "5d847f26b53a30b18ef3f830e5541de9": "positive",
          "ce6054643706390cfe51b6d0cd97abf0": "positive",
          "e98d611d360ab93e31184d8c4bf35e2b": "positive"
      },
      "gold_digest-details-sk_right-overridden": {
          "28041900cd78d4412966eec937575731": "positive",
          "2c56f8826c8717d8c65bba1a074c322e": "positive",
          "93efb7a213223fbde7c306302b9760b1": "positive",
          "a37cb1ddcbc4dd8ecbeec6caff7946ea": "positive"
      },
      "gold_edit-ignore-rule-sk": {
          "934473d79763c47f960bb10948971a31": "positive",
          "ed06feeef1d1f0ff50bdc7313d3c12c7": "positive"
      },
      "gold_edit-ignore-rule-sk_missing-custom-value": {
          "aa8abf76416d4f5456c7360361b81e87": "positive",
          "d3c07dd72969daa68caaa1ccbc2ccb18": "positive"
      },
      "gold_edit-ignore-rule-sk_missing-data": {
          "9ced2802dc4cb456b3af65ff74d84337": "positive",
          "a9ffb9ab2cf08027e9655af4ef813df4": "positive"
      },
      "gold_edit-ignore-rule-sk_with-data": {
          "346e57ea966cfd537c1e1dae926f4f79": "positive"
      },
      "gold_filter-dialog-sk": {
          "0280f73d7673fa1c4898353ff5c5e7dc": "positive",
          "2d4befa5ab4a46f88556f4cf22ff66c1": "positive",
          "979aa649a75f454a0c6dc7ab156354be": "positive"
      },
      "gold_filter-dialog-sk_query-dialog-open": {
          "5cceafabc04142cd609fa245d51e5dc1": "positive",
          "79a207a3d98d6dde845619e53ca577f9": "positive",
          "f59de92092d8313483b9118244473b42": "positive"
      },
      "gold_ignores-page-sk_create-modal": {
          "5aa63a6fa321ba696c8f47c4758a10d3": "positive",
          "ea340245f31b40f23bb6d255d504db7a": "positive"
      },
      "gold_ignores-page-sk_update-modal": {
          "b608d6f2b679db35e4229a8ea1246d92": "positive",
          "fb292ed5d86c6305cd62c247a7a5abc7": "positive"
      },
      "gold_list-page-sk_query-dialog": {
          "1caf1da205f7e4e855b1d20905ec4bee": "positive",
          "5c92542f830edf0474f72673d5da6a15": "positive",
          "f2eb2dd09401fd7d77c3eec3bc159c2f": "positive"
      },
      "gold_query-dialog-sk_key-and-value-selected": {
          "0307e2a9286fc2b8159d2ba3a16e9dc7": "positive",
          "51a4276aec7c922ded06d866d986bb10": "positive"
      },
      "gold_query-dialog-sk_key-selected": {
          "d31dd07deadd75df036df7ae16c2c67d": "positive"
      },
      "gold_query-dialog-sk_multiple-values-selected": {
          "c1c677033f849a951610281a3caf3c78": "positive"
      },
      "gold_query-dialog-sk_no-selection": {
          "d8eddcc865ec838f247dae75914701f3": "positive",
          "ecd2352b87121127698d83c78e761bc6": "positive"
      },
      "gold_query-dialog-sk_nonempty-initial-selection": {
          "a0d92dbe6b7cd6e7e9ee0f6f1e338082": "positive",
          "c953f60c7302d06b157b3f88dd5624ed": "positive",
          "d4c89fc2c41e1077eb56903541c5de25": "positive"
      },
      "gold_search-controls-sk": {
          "548551fe95eeb01bbbf3be044035b9c6": "positive",
          "a521d05ee320e76c740cd69c6a7ec0cb": "positive",
          "e49b6e6e3f06ac9866b6602e6a98e910": "positive"
      },
      "gold_search-controls-sk_left-hand-trace-filter-editor": {
          "2e1a88b33a407e1f2254e58d02ee0057": "positive",
          "3f3d694aa19033bee7a1219511768aa8": "positive",
          "cc5bc3c84cf70a8e109f591b7cf39ccb": "positive"
      },
      "gold_search-controls-sk_more-filters": {
          "1bea9fc10636acf7ae5a2b3cc2c9ced6": "positive",
          "4d9f238c419a4342556a788718282fa7": "positive",
          "ae8e679df1b416ca5bf1d793bca8e3dc": "positive"
      },
      "gold_search-controls-sk_right-hand-trace-filter-editor": {
          "5d8c80eda80e015d633a4125ab0232dc": "positive",
          "d20f37006e436fe17f50ecf49ff2bdb5": "positive",
          "fbd3de3fff6b852ae0bb6751b9763d27": "positive"
      },
      "gold_trace-filter-sk_nonempty": {
          "1ef7f0dd6f83c7ee916dad8596aca631": "positive",
          "ebcb48f48fffa6f584b9b939d6bcafd0": "positive",
          "ffadc0cf71439e9d9e5004f0f51a3053": "positive"
      },
      "gold_trace-filter-sk_nonempty_query-dialog-open": {
          "7a0f953d83033d05c017c0edca73d0c1": "positive",
          "9ec3918a8a7e443c917f2c16a4c90e53": "positive"
      },
      "gold_trace-filter-sk_query-dialog-open": {
          "3633e878bc13e0365144132b8156b0f0": "positive"
      },
      "infra-sk_autogrow-textarea-sk_filled": {
          "465ef8c65d3881403a6862168960752a": "positive",
          "e774564e4b6443f0e95a080b0827ab56": "positive"
      },
      "infra-sk_autogrow-textarea-sk_grow": {
          "d313a417d34dde8093aba66b964e8d6e": "positive",
          "e70554b623c7ca35ae72320f1d8c510e": "positive"
      },
      "infra-sk_autogrow-textarea-sk_shrink": {
          "239b815d1b1ea33454696a9e1e6e07f2": "positive",
          "e5010c288a98d383eb457157b608a8bf": "positive"
      },
      "infra-sk_expandable-textarea-sk_closed": {
          "53476d5ff8dfc87986f227dfb3767937": "positive",
          "e574e02cfe18b44e1d4f098850d4dc03": "positive"
      },
      "infra-sk_expandable-textarea-sk_open": {
          "5d992befcadcaf7461ba435975fa11e4": "positive",
          "78e249a7ad42ca936437a60673953b7b": "positive",
          "d0868fbcf15879c2c9f41d9f19c249df": "positive"
      },
      "perf_alert-config-sk": {
          "2fa58aa430e9c815755624ca6cca4a72": "positive",
          "ed4a8cf9ea9fbb57bf1f302537e07572": "positive"
      },
      "perf_alerts-page-sk": {
          "6971d6faca77f9ddb05a4ae9243127c3": "positive",
          "7eac1eebcc2250813ae69f2fbf3fdefd": "positive",
          "aa697601f254247b9dd904c5efa6f132": "positive",
          "b05f062cdd0a52bc68a64c3cb3b0c808": "positive",
          "b627c19049d4430ca951971ce2850732": "positive",
          "e7a14dd2617ddc77feeef0ec808a0e19": "positive"
      },
      "perf_algo-select-sk": {
          "359fa85bf4bc29b22bea9bd54848f077": "positive",
          "91669ffa41af4cda1dc1548b8845bfb2": "positive",
          "cb121f6f314815f2c4d8c4258c72ca45": "positive"
      },
      "perf_day-range-sk": {
          "880cbde400d0f3c6b864afa3e1397fe0": "positive",
          "e065e221cbd63a5a1a495c985afafe8d": "positive"
      },
      "perf_query-chooser-sk": {
          "45f79b03570eb6e2d85032ae65ae77fe": "positive",
          "772b2d192a55acb5dc9369bda124bd19": "positive"
      }
  }
};
