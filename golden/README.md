Gold Correctness
================

For information on setting up or using Gold, see [these docs](docs/README.md).

For an architectural overview, see:
<https://docs.google.com/document/d/1U7eBzYrZCPx24Lp9JH2scKj3G8Gr8GRtQJZRhdigyRQ/edit>

To run Gold locally, see:
<https://skia.googlesource.com/infra-internal/+show/c6fad0bec78c6768ce7e4187606325216dd438ed/scripts/start-gold-chrome-gpu.sh>

Working on the lit-html frontend
--------------------------------

Gold follows the [pulito](https://www.npmjs.com/package/pulito) model of organization
and uses [webpack](https://webpack.js.org/) to bundle the pages together.

To get started, run `npm install` in the top `golden` directory. You won't need to run
this very often - only when deps are rolled forward.


The [lit-html](https://github.com/Polymer/lit-html) pages are in `./pages`.
These are very simple, as they compose more complex modules found in `./modules`.
To access the demo pages for the modules, run

	make serve

Then open a web browser to [localhost:8080/dist/[module].html](localhost:8080/dist/gold-scaffold-sk.html).
These demo pages have some mock data (piped in via a mock-fetch) and are good proxies for
working with real data from a real web server.

The pages in ./pages also show up at [localhost:8080/dist/[page].html](localhost:8080/dist/changelists.html)
although these won't be as interesting as there is no mock data and you may see strange
artifacts like `{{.Title}}` as that's where the golang templating on the server will insert
data.

As we transition off of Polymer-based pages, there is a "page" called transitional that
houses a wide assortment of lit-html elements that are bundled into the Polymer pages.

To run the tests for these lit-html pages, run:

	make js-test

If you want a browser window left open to inspect the output (e.g. tests are failing):

	make js-test-debug

Tests tend to make sure that data processing and html structure is done correctly.
CSS and "how it looks" are presumed to be handled by a human during
development and integration

If you are running a skiacorrectness server locally, you can (in another terminal)
run

	make frontend

which will rebuild all the frontend pages. When in --local mode, the skiacorrectness
server will reload the templates/pages every time, so you don't have to restart it to
see the re-built pages.

Backend Storage
---------------

Gold uses [CockroachDB](https://www.cockroachlabs.com/get-cockroachdb/) to store all data necessary
for running the backend servers. (Caveat: We are in the middle of a migration towards this goal.)

For production-specific advice, see docs/PROD.md.
