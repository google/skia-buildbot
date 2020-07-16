# The Alert API

Alerts can be edited through the HTTP/JSON API that the Perf UI also uses.

| URL                           | Method | Request | Response | Notes                                                       |
| ----------------------------- | ------ | ------- | -------- | ----------------------------------------------------------- |
| `/_/alert/list/`              | GET    |         | []Alert  |                                                             |
| `/_/alert/list/true`          | GET    |         | []Alert  | Returns deleted alerts also.                                |
| `/_/alert/new`                | GET    |         | Alert    | A pre-populated Alert with the instance defaults filled in. |
| `/_/alert/update`             | POST   | Alert   |          | 200 OK on success.                                          |
| `/_/alert/delete/{id:[0-9]+}` | POST   |         |          | 200 OK on success.                                          |

See [/json/index.ts](./modules/json/index.ts) for the TypeScript definition of Alert.

To create a new alert, first GET a pre-populated Alert from `/_/alert/new`, make modifications to the Alert
and then POST that modified Alert back to `/_/alert/update`. Since the pre-populated Alert has an 'id' of -1
the server will know that the POST is a request to create a new Alert.

If your instance of Perf is protected by authentication then you will also need to supply
credentials on the 'update' and 'delete' requests, which can be done by providing
an OAuth2 Bearer Token in an Authorization: header. For example:

    Authorization: Bearer 1234567890

One easy way to get such a token is via the 'gcloud' command line:

    gcloud auth print-access-token
