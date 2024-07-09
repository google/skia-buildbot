This directory contains the frontend service implementation and it's respective controllers.

The frontend service is intended to host api endpoints that are user facing and invoked from the UI.

[frontend.go](./frontend.go) is the entry point to the service and defines what apis are to be
hosted. It also creates the necessary objects (store implementations, data/feature providers, etc)
that are required by the individual apis and passes them on when creating the api instances.

[api](./api/) directory contains individual apis that have a struct per category. Below are the
currently supported apis.

| API                                    | Description                                                                                    |
| -------------------------------------- | ---------------------------------------------------------------------------------------------- |
| [Alerts](./api/alertsApi.go)           | Contains endpoints related to regression alert configurations                                  |
| [Favorites](./api/favoritesApi.go)     | Contains endpoints for the Favorites feature                                                   |
| [Graphs](./api/graphApi.go)            | Contains endpoints that provide data and trigger actions related to plotting individual graphs |
| [Pinpoint](./api/pinpointApi.go)       | Contains endpoints related to pinpoint jobs                                                    |
| [Query](./api/queryApi.go)             | Contains endpoints that serve the query dialog                                                 |
| [Regressions](./api/regressionsApi.go) | Contains endpoints related to regressions detected by the system                               |
| [Shortcuts](./api/shortcutsApi.go)     | Contains endpoints that create and manage shortcuts                                            |

# Adding a new endpoint

1. Check if the endpoint you are adding falls into any of the categories described above.
2. If yes, add your handler function for the endpoint in the respective file and register the http
   route for that endpoint in the `RegisterHandlers` function in that file.
3. If no, create a new file and define a struct for the api. The struct needs to implement
   the [FrontendApi interface](./api/api.go), so add the `RegisterHandlers` implementation and add
   the routes and their corresponding handlers. Once ready, add your struct implementation to the
   list in `getFrontendApis()` in [frontend.go](./frontend.go).
