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
  extends: ['prettier'],
  globals: {
    Atomics: 'readonly',
    SharedArrayBuffer: 'readonly',
  },
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 11,
    sourceType: 'module',
  },
  ignorePatterns: ['dist/', 'build/', '_bazel*', 'new_element/templates/'],
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
    'no-restricted-syntax': ['off'],
    'no-return-assign': ['off'],
    'no-shadow': ['warn'],
    'no-underscore-dangle': ['off'],
    'no-use-before-define': ['warn', { functions: false, variables: false }],
    'object-shorthand': ['off'],
    'prefer-destructuring': ['off'],
    'prefer-object-spread': ['off'],
    'space-before-function-paren': ['off'],
    eqeqeq: ['error'],

    'no-useless-backreference': ['error'],
    'no-useless-call': ['error'],
    'no-useless-catch': ['error'],
    'no-useless-computed-key': ['error'],
    'no-useless-concat': ['error'],
    'no-useless-escape': ['error'],
    'no-useless-rename': ['error'],
    'no-useless-return': ['error'],
    'no-unused-labels': ['error'],
    'no-unused-private-class-members': ['error'],

    // All of these should be turned back to errors once all the instances are
    // found and fixed.
    'prefer-promise-reject-errors': ['warn'],
    radix: ['warn'],
    'no-nested-ternary': ['warn'],
    'no-restricted-properties': ['warn'],
    'no-throw-literal': ['warn'],
    'guard-for-in': ['warn'],
  },
  overrides: [
    {
      files: ['**/*.ts', '**/*.tsx'],
      parser: '@typescript-eslint/parser',
      // Start with the recommended rules, but turn some of them off in the
      // 'rules' section below.
      extends: ['plugin:@typescript-eslint/recommended', 'plugin:lit/recommended'],
      settings: {
        'import/resolver': {
          node: {
            extensions: ['.js', '.jsx', '.ts', '.tsx'],
          },
        },
      },
      plugins: ['@stylistic/eslint-plugin', '@typescript-eslint', 'eslint-plugin-import'],
      rules: {
        // Allow ! non-null assertions.
        '@typescript-eslint/no-non-null-assertion': 'off',

        // go2ts will generate empty interfaces.
        '@typescript-eslint/no-empty-interface': 'off',

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

        '@typescript-eslint/no-empty-function': 'off',
        '@typescript-eslint/no-useless-constructor': ['error'],

        '@stylistic/type-annotation-spacing': [
          'error',
          {
            before: false,
            after: true,
            overrides: {
              arrow: {
                before: true,
                after: true,
              },
            },
          },
        ],

        '@stylistic/lines-between-class-members': [
          'error',
          'always',
          { exceptAfterOverload: true },
        ],

        'space-before-function-paren': 'off',
        '@typescript-eslint/space-before-function-paren': ['off'],

        // We already disallow implicit-any, explicit is fine.
        '@typescript-eslint/no-explicit-any': 'off',

        // All of these should be turned back to errors once all the instances are
        // found and fixed.
        '@typescript-eslint/ban-types': 'off',

        '@typescript-eslint/no-unused-vars': [
          'error',
          {
            args: 'all',
            argsIgnorePattern: '^_',
            caughtErrors: 'all',
            caughtErrorsIgnorePattern: '^_',
            destructuredArrayIgnorePattern: '^_',
            varsIgnorePattern: '^_',
            ignoreRestSiblings: true,
          },
        ],
      },
    },
    {
      files: ['*_test.ts'],
      rules: {
        // Prevents Chai.js assertions such as expect(foo).to.be.true from causing "Expected an
        // assignment or function call and instead saw an expression." linting errors.
        '@typescript-eslint/no-unused-expressions': 'off',
      },
    },
  ],
};
