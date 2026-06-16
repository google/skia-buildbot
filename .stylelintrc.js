module.exports = {
  extends: ['stylelint-config-standard-scss'],
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

    // -- Temporary overrides --
    // TODO(ansid): enable these rules.

    // Needs manual fixes
    'block-no-empty': null,
    'font-family-no-missing-generic-family-keyword': null,
  },
};
