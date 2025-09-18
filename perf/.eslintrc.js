module.exports = {
  ignorePatterns: [
    // These are auto generated
    'modules/json/index.ts',
  ],
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
