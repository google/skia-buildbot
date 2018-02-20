// Helper function when making fetch() requests.
//
// Usage:
//
//    fetch('/_/list').then(jsonOrThrow).then(json => {
//      // Do something with the parsed json here.
//    }).catch(errorMessage);
//
export const jsonOrThrow = (resp) => {
  if (resp.ok) {
    return resp.json();
  }
  throw 'Bad network response.';
}
