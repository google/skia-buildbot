#!/bin/bash
set -e

case "$1" in
  "type")
    typescript-coverage-report \
      --project perf/coverage/tsconfig.coverage.json \
      -o perf/coverage-reports/type-coverage
    ;;
  "test")
    c8 -c perf/coverage/.c8rc.json mocha \
      -r ts-node/register \
      -r perf/mocha-setup.ts \
      "perf/modules/**/*_test.ts" \
      --exclude "perf/modules/**/*_puppeteer_test.ts" \
      --reporter spec \
      --timeout 10000 \
      --exit
    ;;
  "mutation")
    stryker run perf/coverage/stryker.config.json
    ;;
  "all")
    npm run perf-type-coverage
    npm run perf-test-coverage
    npm run perf-mutation-testing
    python3 perf/coverage/add-coverage-links.py
    ;;
  *)
    echo "Usage: $0 {type|test|mutation|all}"
    exit 1
    ;;
esac
