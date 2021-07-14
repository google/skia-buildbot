Skia Commit Queue (SkCQ)
========================

Design doc is [here](https://goto.google.com/skia-commit-queue).

Working on the lit-html frontend
--------------------------------

SkCQ follows the [pulito](https://www.npmjs.com/package/pulito) model of organization
and uses [webpack](https://webpack.js.org/) to bundle the pages together.

To get started, run `npm ci` in the repository's root directory. You won't need to run
this very often - only when deps are rolled forward.


The [lit-html](https://github.com/Polymer/lit-html) pages are in `./pages`.
These are very simple, as they compose more complex modules found in `./modules`.
To access the demo pages for the modules, run

        make serve

Then open a web browser to [localhost:8080/dist/[module].html](localhost:8080/dist/skcq-scaffold-sk.html).
These demo pages have some mock data (piped in via a mock-fetch) and are good proxies for
working with real data from a real web server.

The pages in ./pages also show up at [localhost:8080/dist/[page].html](localhost:8080/dist/changelists.html)
although these won't be as interesting as there is no mock data and you may see strange
artifacts like `{{.Title}}` as that's where the golang templating on the server will insert
data.

