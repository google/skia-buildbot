import { binarySearch } from './binarySearch.js'

// a little helper to self-document the conversion.
function indexToNotFound(i) {
    if (i < 0) {
    assert.fail(` index ${i} should be >= 0`);
  }
  return -1 - i;
}

describe('binarySearch', function(){
  it('handles empty array', function(){
    assert.equal(indexToNotFound(0), binarySearch([], 7));
    assert.equal(indexToNotFound(0), binarySearch([], 'a'));
  });

  it('handles singleton arrays', function(){
    assert.equal(indexToNotFound(1), binarySearch([0], 7));
    assert.equal(indexToNotFound(0), binarySearch([0], -7));
    assert.equal(                0 , binarySearch([0], 0));

    assert.equal(indexToNotFound(1), binarySearch(['b'], 'c'));
    assert.equal(indexToNotFound(0), binarySearch(['b'], 'a'));
    assert.equal(                0 , binarySearch(['b'], 'b'));
  });

  it('is flexible with duplicate values', function(){
    assert.equal(indexToNotFound(4), binarySearch([1,3,3,6], 7));
    assert.equal(                0 , binarySearch([1,3,3,6], 1));
    assert.equal(                3 , binarySearch([1,3,3,6], 6));
    assert.equal(indexToNotFound(3), binarySearch([1,3,3,6], 5));
    let idx = binarySearch([1,3,3,6], 3);
    if (!(idx === 1 || idx === 2)) {
      assert.fail(idx, '1 or 2', `${idx} should be 1 or 2`);
    }
  });

  it('agrees with standard indexOf', function(){
    let arr = [];
    for (let i = 0; i <= 100; i++) {
      if (i % 5 === 0) {
        arr.push(i);    // duplicate multiples of 5
      } else if (i % 3 === 0) {
        continue; // omit multiples of 3 that are not a multiple of 5
      }
      arr.push(i);
    }

    for (let i = 0; i <= 100; i++) {
      if (i % 5 === 0) {
        // indexOf returns the first index of the item.
        let first = arr.indexOf(i);
        let idx = binarySearch(arr, i);
        if (!(idx === first || idx === (first+1))) {
          assert.fail(idx, `${i} or ${i+1}`, `Broken for i=${i} (duplicates)`);
        }
      } else if (i % 3 === 0) {
        // For not found things, look for element one higher (which is found,
        // by construction of the test array) and know that we should insert it
        // at that index.
        assert.equal(indexToNotFound(arr.indexOf(i+1)), binarySearch(arr, i),
                     `broken for i=${i} (div by 3)`)
      } else {
        assert.equal(arr.indexOf(i), binarySearch(arr, i), `Broken for i=${i}`);
      }
    }
  });
});