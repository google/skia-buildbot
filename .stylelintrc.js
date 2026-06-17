module.exports = {
  extends: ['stylelint-config-standard-scss'],
  ignoreFiles: [
    'new_element/templates/**',
    'new_element/modules/**',
    '_bazel_bin/**',
    '_bazel_buildbot/**',
    '_bazel_out/**',
    '_bazel_testlogs/**',
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
  ],
};
