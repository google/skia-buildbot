# The Cron Service

The Cron Service periodically makes URL fetches, runs Swarming tasks or DM
quests. It uses luci-config to fetch per-project lists of cron jobs. It tries
to prevent concurrent execution of cron job invocation (i.e. an invocation will
not start if previous one is still running).

It's built on top of App Engine Task Queues service.

## Terminology

To reduce confusion:

*   __Cron job__ or just __job__ is a definition of the periodic activity that
    Cron service consumes.
*   __Invocation__ is an actual attempt to execute an activity specified by a
    job. Invocation has duration in time. It starts, runs, and completes. One
    example is Swarming tasks.
*   __Task queue task__ or just __task__ is a GAE Task Queue Service task.
    An act of enqueuing a task with some non-default ETA is referred to as
    "scheduling".
*   __GAE cron task__ is GAE Cron Service task (defined via `cron.yaml`).

## Design overview

A cron job state is stored in datastore in a separate entity group. It is
updated in a chain of Task Queue tasks. The lifecycle of a cron job:

1.  A job is registered with the service. It is set to state `SCHEDULED` and
    first `TickLater` task is scheduled to run at some time in the future
    (based on cron job schedule).
1.  `TickLater` runs. It transactionally updates job's state to `QUEUED`,
    schedules next `TickLater` task and enqueues `StartInvocation` task.
1.  `StartInvocation` task launches the invocation (starts URL fetch, Swarming
    task, etc) and moves the job to `RUNNING` state. Once the invocation is
    finished, the job moves back to `SCHEDULED` state.

See `statemachine.go` for complete description of all various states.

## Handling internal failures

The Cron Service relies on two GAE subsystems: Datastore Service and Task Queue
Service. There are some associated concerns:

*   Datastore can effectively become read only for extended periods of time if
    underlying GAE infrastructure is under stress (e.g. the app is being
    migrated to another datacenter).
*   Task queue service's storage should be considered ephemeral. Task queues can
    be purged via admin console or somehow otherwise "drained" (imagine a bug
    in the code that causes service to consume tasks, but do wrong things).
    Since `TickLater` tasks are chained, a single skipped task may stop
    processing of some cron job forever.

Datastore partial availability problem is tricky because naive implementation
may choose to retry `StartInvocation` tasks due to failed datastore writes and
accidentally launch many invocations instead of one. Imagine the service
scheduling a storm of Swarming tasks or DM quests, overloading entire
infrastructure, just because its datastore is having a bad day.

To workaround datastore partial availability the service always writes something
to datastore before sending external requests. In that case, if datastore is
having issues, they will be detected before an external service is hit.

To workaround task queue issue the service uses "watchdog" GAE cron task:

1.  When `TickLater` or `StartInvocation` task is being scheduled, the cron job
    entity is updated with `WatchdogTimerTs`: a timestamp in the future when
    the task should be finished already.
1.  When `TickLater` or `StartInvocation` are running when expected, they move
    that timestamp further.
1.  Separate "watchdog" GAE cron once per minute fetches from datastore all
    cron jobs that have `WatchdogTimerTs` less than `Now()` and repairs their
    state (by launching another `TickLater` task) or at least reports them.
1.  Since watchdog triggering is considered an exceptional situation, the query
    above should return small number of entities, and thus single cron job
    should be able to handle them all within request deadline.
