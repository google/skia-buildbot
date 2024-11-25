import { expect } from 'chai';
import { truncate } from './string';

const test = (input: string, len: number, output: string, msg?: string | undefined) => {
  expect(truncate(input, len), msg).to.equal(output);
};

describe('truncate', () => {
  it('handles small len', () => {
    test('', 2, '');
    test('a', 2, 'a');
    test('ab', 2, 'ab');
    test('abc', 2, 'ab');
    test('abcd', 2, 'ab');
    test('abcde', 2, 'ab');
    test('abc', 3, 'abc');
    test('abcd', 3, 'abc');
  });

  it('handles invalid len', () => {
    test('abcde', 0, '');
    test('abcde', -1, '');
    test('abcde', -449, '');
  });

  it('replaces the end of longer strings with ellipses', () => {
    test('abcde', 4, 'a...');
    test('abcdefghijkl', 9, 'abcdef...');
  });

  it("doesn't touch strings which are already short enough", () => {
    test('abcd', 4, 'abcd');
    test('abcde', 6, 'abcde');
  });
});
