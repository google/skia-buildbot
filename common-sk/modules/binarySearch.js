/* Returns the index of i in arr or a negative value if not found.
 * The negative value can be converted into 'index this element should
 * go' by doing -1 - idx;
 * Requires arr to be sorted. */
export function binarySearch(arr, i) {
  let left = 0;
  let right = arr.length - 1;

  while (left < right) {
    let c = Math.floor((left + right) / 2); // c is for center
    if (arr[c] === i) {
      return c;
    }
    if (i < arr[c]) {
      right = c - 1;
    } else {
      left = c + 1;
    }
  }
  if (arr[left] === i) {
    return left;
  } else if (arr[left] < i) {
    return -2 - left;
  } else {
    return -1 - left;
  }
}
