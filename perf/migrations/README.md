# SQL Migrations

Pref can use SQL backends to store both trace data and other data like shortcuts
and alerts. We need to have a system in place that allows changing the schema of
the database and also upgrading an existing database to the schema that the
version of Perf expects. To do that we use the
[`github.com/golang-migrate/migrate/v4`](https://pkg.go.dev/github.com/golang-migrate/migrate/v4?tab=overview)
library.

Each directory below here represents a supported SQL dialect. Note that the
directory names should match the values of sql.Dialect. Each directory contains
a series of SQL files that migrate the database schema from one version to the
next. The version number is the 0-padded prefix of each file. Also note that
there is both an `.up.` and `.down.` version of each migration, so we can
smoothly migrate in both directions. There are also tests that ensure that the
migrations work in both directions.