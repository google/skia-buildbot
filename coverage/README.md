The coverage server has two purposes:
	- Allow users to browse the test coverage of Skia on recent commits.
	- Provide an endpoint for integration with Gerrit so coverage can be displayed from tryjobs.

It works largely by ingesting data from GCS from coverage runs.
