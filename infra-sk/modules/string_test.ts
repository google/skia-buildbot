import { expect } from 'chai';
import { truncate } from './string';

const test = (input: string, len: number, output: string, msg?: string | undefined) => {
    expect(truncate(input, len), msg).to.equal(output);
};

describe("truncate", function() {
  it("handles small len", function() {
    test("", 2, "");
    test("a", 2, "a");
    test("ab", 2, "ab");
    test("abc", 2, "ab");
    test("abcd", 2, "ab");
    test("abcde", 2, "ab");
    test("abc", 3, "abc");
    test("abcd", 3, "abc");
  });

  it("handles invalid len", function() {
    test("abcde", 0, "");
    test("abcde", -1, "");
    test("abcde", -449, "");
  });

  it("replaces the end of longer strings with ellipses", function() {
    test("abcde", 4, "a...");
    test("abcdefghijkl", 9, "abcdef...");
  });

  it("doesn't touch strings which are already short enough", function() {
    test("abcd", 4, "abcd")
    test("abcde", 6, "abcde");
  });
});
