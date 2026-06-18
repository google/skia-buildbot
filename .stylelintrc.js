module.exports = {
  extends: ['stylelint-config-standard-scss'],
  ignoreFiles: [
    'new_element/templates/**',
    'new_element/modules/**',
    '_bazel_bin/**',
    '_bazel_buildbot/**',
    '_bazel_out/**',
    '_bazel_testlogs/**',
    '**/*.ts',
    '!perf/**/*-sk.ts',
    '!perf/**/*.css.ts',
    // Exclude v-resizable-box-sk.ts because postcss-lit corrupts dynamic template
    // literal expressions like ${fontPosition.upper}px into postcss_lit_0px during --fix.
    'perf/modules/plot-google-chart-sk/v-resizable-box-sk.ts',
  ],
  rules: {
    // -- Permament overrides --
    // Disable strict kebab-case naming (allow historical Skia naming)
    'scss/dollar-variable-pattern': null,
    'scss/at-mixin-pattern': null,
    'scss/at-function-pattern': null,
    'selector-class-pattern': null,
    'keyframes-name-pattern': null,
    'selector-id-pattern': null,

    // Allow empty -demo.scss files
    'no-empty-source': null,

    // Allow the Angular "ng-deep" pseudo-element.
    'selector-pseudo-element-no-unknown': [
      true,
      {
        ignorePseudoElements: ['ng-deep'],
      },
    ],
  },
  overrides: [
    {
      files: ['perf/**/*.scss'],
      rules: {
        // Hardcoded colors prevent light/dark theming. Use theme vars instead.
        'color-no-hex': true,
        'color-named': 'never',
        'function-disallowed-list': ['rgb', 'rgba', 'hsl', 'hsla'],
      },
    },
    {
      files: ['perf/**/*-sk.ts', 'perf/**/*.css.ts'],
      customSyntax: 'postcss-lit',
      rules: {
        // Hardcoded colors prevent light/dark theming. Use theme vars instead.
        'color-no-hex': true,
        'color-named': 'never',
        'function-disallowed-list': ['rgb', 'rgba', 'hsl', 'hsla'],
      },
    },
    {
      // -- Gradual Color Migration Exceptions --
      // Components currently containing raw color fallbacks or hardcoded colors.
      // Remove files from this list as their color formatting is migrated.
      files: [
        'perf/modules/explore-multi-v2-sk/explore-multi-v2-sk.ts',
        'perf/modules/explore-multi-v2-sk/explore-toolbar-sk.ts',
        'perf/modules/explore-multi-v2-sk/help-hub-sk.ts',
        'perf/modules/explore-multi-v2-sk/interactive-tour-sk.ts',
        'perf/modules/explore-multi-v2-sk/multi-select-sk.ts',
        'perf/modules/explore-multi-v2-sk/plot-summary-v2-sk.ts',
        'perf/modules/explore-multi-v2-sk/query-bar-sk.ts',
        'perf/modules/explore-multi-v2-sk/trace-chart-sk.ts',
        'perf/modules/explore-multi-v2-sk/trace-chart-tooltip-sk.ts',
        'perf/modules/gemini-side-panel-sk/gemini-side-panel-sk.ts',
        'perf/modules/plot-google-chart-sk/drag-to-zoom-box-sk.ts',
        'perf/modules/plot-google-chart-sk/plot-google-chart-sk.ts',
        'perf/modules/plot-google-chart-sk/side-panel-sk.ts',
        'perf/modules/user-issue-sk/user-issue-sk.ts',
      ],
      customSyntax: 'postcss-lit',
      rules: {
        'color-no-hex': null,
        'color-named': null,
        'function-disallowed-list': null,
      },
    },
  ],
};
