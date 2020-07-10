Ingestion V2
============

Version 2 of the ingestion frameworks aims at being more generic to make it
easier to implement a new ingestion approach and easier to adapt.

At one point, it was used by both Gold and Perf, but the nuances of the two
systems made them diverge on implementation.

We have codified the new ingestion approach as interfaces in ingestion.go.

Key concepts
------------

* Source: anything that can produce lists of ResultFiles (see below).
  The result files are either produced by polling the source or via
  events that are issued by the source. Any source is only required
  to implement the polling method, but can provide a non-functional
  no-op implementation of the events channel.

* Processor: processes a list of result files and (most often) stores them
   in some data store.

* ResultFile: is an abstract interface to a file. It leaves it open where the
  file is stored.
