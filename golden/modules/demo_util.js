// delay wraps the value toReturn in a Promise that will resolve
// after "delay" milliseconds.
export function delay(toReturn, delay=100) {
  // we return a function that returns the promise so each call
  // has a "fresh" promise and waits for the time.
  return function() {
    return new Promise((resolve) => {
      setTimeout(resolve, delay);
    }).then(() => {
      return {
        status: 200,
        body: JSON.stringify(toReturn),
        headers: {'content-type':'application/json'},
      };
    });
  };
}
