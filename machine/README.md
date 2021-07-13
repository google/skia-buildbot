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

See the [Design Doc](http://go/skia-switchboard).
