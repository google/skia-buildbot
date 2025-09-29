module.exports = {
  ignorePatterns: [
    // These are auto generated
    'json/index.ts',
  ],
  parserOptions: {
    // Root tsconfig.json does not include any source files,
    // so we need to include them manually.
    project: './tsconfig.eslint.json',
    tsconfigRootDir: __dirname,
  },
  rules: {
    // Rules that improve async safety and prevent common bugs.
    // TODO(ansid): turn this to errors once all current occurences are fixed.
    '@typescript-eslint/no-floating-promises': 'warn',
    '@typescript-eslint/no-misused-promises': 'warn',
    '@typescript-eslint/await-thenable': 'warn',
    '@typescript-eslint/promise-function-async': 'warn',
    '@typescript-eslint/return-await': ['warn', 'always'],
  },
};
