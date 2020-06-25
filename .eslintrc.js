// To make use of this eslint config file run:
//
//   npm ci
//
// Then install eslint support in your IDE of choice.
module.exports = {
  root: true,
  env: {
    browser: true,
    es6: true,
  },
  extends: [
    'airbnb-base',
  ],
  globals: {
    Atomics: 'readonly',
    SharedArrayBuffer: 'readonly',
  },
  parserOptions: {
    ecmaVersion: 2018,
    sourceType: 'module',
  },
  rules: {
    'camelcase': ['off'],
    'class-methods-use-this': ['off'],
    'func-names': ['off'],
    'import/prefer-default-export': ['off'],
    'max-classes-per-file': ['off'],
    'max-len': ['off'],
    'no-alert': ['off'],
    'no-bitwise': ['warn'],
    'no-continue': ['off'],
    'no-lone-blocks':['off'],
    'no-param-reassign': ['off'],
    'no-plusplus': ['off'],
    'no-restricted-syntax': ['warn'],
    'no-return-assign': ['off'],
    'no-shadow': ['warn'],
    'no-underscore-dangle': ['off'],
    'no-use-before-define': ['error', { 'functions': false, 'variables': false }],
    'object-shorthand': ['off'],
    'prefer-destructuring': ['off'],
    'prefer-object-spread': ['off'],
    'space-before-function-paren': ['error', { 'anonymous': 'never', 'named': 'never', 'asyncArrow': 'always' }],
  },
};
