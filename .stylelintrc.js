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

    // -- Temporary overrides --
    // TODO(ansid): enable these rules.

    // Autofixable with `stylelint --fix`
    'rule-empty-line-before': null,
    'declaration-empty-line-before': null,
    'at-rule-empty-line-before': null,
    'scss/double-slash-comment-empty-line-before': null,
    'comment-whitespace-inside': null,
    'length-zero-no-unit': null,
    'color-function-alias-notation': null,
    'color-function-notation': null,
    'alpha-value-notation': null,
    'shorthand-property-no-redundant-values': null,
    'declaration-block-no-redundant-longhand-properties': null,
    'media-feature-range-notation': null,
    'font-family-name-quotes': null,
    'import-notation': null,
    'block-no-empty': null,

    // Needs manual fixes
    'scss/load-partial-extension': null,
    'font-family-no-missing-generic-family-keyword': null,
    'declaration-block-no-duplicate-properties': null,
    'property-no-deprecated': null,
  },
};
