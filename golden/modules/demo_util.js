// delay wraps the value toReturn in a Promise that will resolve
// after "delay" milliseconds.
export function delay(toReturn, delay=100) {
  return new Promise((resolve) => {
    setTimeout(resolve, delay);
  }).then(() => {
    return {
      status: 200,
      body: JSON.stringify(toReturn),
      headers: {'content-type':'application/json'},
    };
  });
}