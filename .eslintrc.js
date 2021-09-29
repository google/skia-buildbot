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
  extends: ['airbnb-base'],
  globals: {
    Atomics: 'readonly',
    SharedArrayBuffer: 'readonly',
  },
  parserOptions: {
    ecmaVersion: 2018,
    sourceType: 'module',
  },
  ignorePatterns: ['dist/', 'build/', '_bazel*'],
  rules: {
    camelcase: ['off'],
    'class-methods-use-this': ['off'],
    'func-names': ['off'],
    'import/prefer-default-export': ['off'],
    'max-classes-per-file': ['off'],
    'max-len': ['off'],
    'no-alert': ['off'],
    'no-bitwise': ['warn'],
    'no-continue': ['off'],
    'no-lone-blocks': ['off'],
    'no-param-reassign': ['off'],
    'no-plusplus': ['off'],
    'no-restricted-syntax': ['warn'],
    'no-return-assign': ['off'],
    'no-shadow': ['warn'],
    'no-underscore-dangle': ['off'],
    'no-use-before-define': ['error', { functions: false, variables: false }],
    'object-shorthand': ['off'],
    'prefer-destructuring': ['off'],
    'prefer-object-spread': ['off'],
    'space-before-function-paren': [
      'error',
      { anonymous: 'never', named: 'never', asyncArrow: 'always' },
    ],
  },
  overrides: [
    {
      files: ['*.ts', '*.tsx'],
      parser: '@typescript-eslint/parser',

      // Start with the recommended rules, but turn some of them off in the
      // 'rules' section below.
      extends: ['plugin:@typescript-eslint/recommended'],
      settings: {
        'import/resolver': {
          node: {
            extensions: ['.js', '.jsx', '.ts', '.tsx'],
          },
        },
      },
      plugins: ['@typescript-eslint'],
      rules: {
        // Allow ! non-null assertions.
        '@typescript-eslint/no-non-null-assertion': 'off',

        // Require a consistent member declaration order
        '@typescript-eslint/member-ordering': 'off',

        // Don't require the .ts extension for imports.
        'import/extensions': 'off',

        // Sometimes we need to import an interface, but also we need the
        // side-effects to run, e.g. to register an element, which requires two
        // import statements.
        //
        // https://github.com/Microsoft/TypeScript/wiki/FAQ#why-are-imports-being-elided-in-my-emit
        // https://github.com/microsoft/TypeScript/issues/9191
        'import/no-duplicates': 'off',

        // a: string = 'foo' might be redundant, but it's not harmful.
        '@typescript-eslint/no-inferrable-types': 'off',

        '@typescript-eslint/type-annotation-spacing': [
          'error',
          {
            before: false,
            after: true,
          },
        ],

        // note you must disable the base rule as it can report incorrect errors
        'lines-between-class-members': 'off',
        '@typescript-eslint/lines-between-class-members': [
          'error',
          'always',
          { exceptAfterOverload: true },
        ],

        // note you must disable the base rule as it can report incorrect errors
        'space-before-function-paren': 'off',
        '@typescript-eslint/space-before-function-paren': [
          'error',
          {
            anonymous: 'never',
            named: 'never',
            asyncArrow: 'always',
          },
        ],

        // We already disallow implicit-any, explicit is fine.
        '@typescript-eslint/no-explicit-any': 'off',
      },
    },
  ],
};
