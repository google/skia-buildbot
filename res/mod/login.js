// A Promise that will be resolved with the users current login status.
//
// The resolution object looks like:
//
//   {
//     "Email": "fred@example.com",
//     "LoginURL": "https://..."
//   }
//
// The Email will be the empty string if the user is not logged in.
export var login = fetch('/loginstatus/').then(res => {
  if (res.ok) {
    return res.json()
  }
  throw new Error('Problem reading /loginstatus/:' + res.statusText);
});
