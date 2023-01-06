# Machine Server

The machine state server is a centralized management application for device
testing.

See the [Design Doc](http://go/skolo-machine-state).

## Code structure

The main code is structure as:

    go/machine/
        source/
        processor/
        store/

Where:

- `types` contains the Go types used across the rest of the modules.
- The `source` module contains `source.Source`, a way to get update events from
  machines.
- The `store` module contains `store.Store`, a way to persist and retrieve each
  machine's state.
- The `processor` module contains `processor.Processor`, a way to update a
  machine state from an incoming event.

The main loop of machine state server looks like:

    for event := range eventCh {
    	store.Update(ctx, event.Host.Name, func(previous machine.Description) machine.Description {
    		return processor.Process(ctx, previous, event)
    	})
    }

## test_machine_monitor

The application that runs on each switchboard test machine and feeds information
into the machine state server.

# Current Data Flow

## User triggers powercycle for a machine.

| initiator                 | message                                 | target        | notes                                          |
| ------------------------- | --------------------------------------- | ------------- | ---------------------------------------------- |
| machineserver             | >Set(Δ[Description][desc])              | DB            | Description.Powercycle=true                    |
| powercycle_server_ansible | <WebAPI([ListPowerCycleResponse][lpcr]) | machineserver | GET on `/json/v1/powercycle/list`              |
| powercycle_server_ansible | >WebAPI                                 | machineserver | POST to `/json/v1/powercycle/complete/{id:.+}` |

## powercycle_server_ansible report availability on startup.

| initiator                 | message                                       | target        | notes                                      |
| ------------------------- | --------------------------------------------- | ------------- | ------------------------------------------ |
| powercycle_server_ansible | >WebAPI([UpdatePowerCycleStateRequest][pssu]) | machineserver | POST to `/json/v1/powercycle/state/update` |

## How test_machine_monitor keeps machine.Description up to date.

| initiator            | message                      | target        | notes                                                            |
| -------------------- | ---------------------------- | ------------- | ---------------------------------------------------------------- |
| test_machine_monitor | >WebAPI([Event][event])      | machineserver | POST to `/json/v1/machine/event`                                 |
| test_machine_monitor | <WebAPI([Description][desc]) | machineserver | GET to `/json/v1/machine/description/{id:.+}`                    |
| test_machine_monitor | <SSE([Description][desc])    | machineserver | GET to `/json/v1/machine/sse/description/updated?stream={id:.+}` |

[desc]:
  https://pkg.go.dev/go.skia.org/infra/machine/go/machine#Description
  'machine.Description'
[event]:
  https://pkg.go.dev/go.skia.org/infra/machine/go/machine#Event
  'machine.Event'
[lpcr]:
  https://pkg.go.dev/go.skia.org/infra/machine/go/machineserver/rpc#ListPowerCycleResponse
  'rpc.ListPowerCycleResponse'
[pssu]:
  https://pkg.go.dev/go.skia.org/infra/machine/go/machineserver/rpc#UpdatePowerCycleStateRequest
  'rpc.UpdatePowerCycleStateRequest'

# Legend

- initiator - The entity that starts the action.
- message - Shows the direction of the data flow, the transport, and the struct
  being moved.
  - `Set` - A database operation
  - `WebAPI` - An HTTP Request
  - `SSE` - Server-Sent Event
  - `PubSub` - A PubSub message.
- target - Who the iniator is talking to.
- Δ is used before a struct if only a part of that struct is being changed.

# Data Storage

Machine server stores its data in CockroachDB.

## Initializing the database.

Run the following commands to initialize to create the database and the initial
tables.

Run this while talking to the correct GKE cluster:

     kubectl port-forward service/machineserver-cockroachdb-public 25001:26257

In a separate shell run:

     bazel run --config=mayberemote //machine/go/machine/store/cdb/mscdbinit:mscdbinit