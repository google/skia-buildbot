# Gold instance configurations

## json5 files

These are the configurations for each Gold instance. They are used by various
Gold services, built into the container, and passed as arguments in the yaml
files in the k8s-config repo.

## CockroachDB data retention policy files

These are SQL files that need to be executed manually to apply data retention
policies on certain tables. We use
[Row-Level TTL](https://www.cockroachlabs.com/docs/stable/row-level-ttl)
to automatically delete expired data.

Data retention policies are implemented as configurations in the data layer
rather than part of the application schema because:

- The policies may vary for each instance.
- Gold does not depend on the policies to function, and should not use those
  columns (`expire_at`).
- This separation of concern allows us to evolve the implementation of data
  retention policies in the future.

Guidelines when defining policies for a specific instance.

- Keep data at least 2x the size of the commits sliding window.
- Define proper policies based on the purpose of a table. For example, the
  `ValuesAtHead` table is a "caching" table that requires a policy similar to
  LRU cache; the `Expectations` table is a "core entity" that should retain all
  triaged records indefinitely and only delete overdue un-triaged ones.
- Refer to existing policies of other instances for techniques and consistency.
