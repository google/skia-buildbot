alert-manager
=============

Alert manager converts Prometheus alerts into PubSub events which
are then used to drive the state in alert-manager.

```
		+-----------------------------+      +--------------------------+
		|  Prometheus (skia|buildbot) |      | Prometheus (skia|public) |
		+-------------+---------------+      +------------+-------------+
									|                                   |
									|                                   |
				 +--------v--------+                 +--------v--------+
				 |                 |                 |                 |
				 | alert-to-pubsub +-------+      +--+ alert-to-pubsub |
				 |                 |       |      |  |                 |
				 +-----------------+       |      |  +-----------------+
																	 |      |
													+--------v------v----------+
													| PubSub                   |
													| Topic: prometheus-alerts |
													+---------------+----------+
																					|
		 +-----------------+                  |
		 |                 |          +-------v-------+
		 | Cloud Datastore <----------+               |
		 |   IncidentAm    |          | alert-manager |
		 |   SilenceAm     +---------->               |
		 |                 |          +---------------+
		 +-----------------+
```

The alert-manager application is state-less, all state is stored
in the Cloud Datastore. Also, all the logic for applying silences
to Incidents is done in the UI, i.e. the alert-manager backend
just reads and writes Incidents and Silences without looking at
the interactions between the two.
