# dot

This is the code for https://dot.skia.org, a service for rendering
Graphviz dot code into SVG.

We look for Graphviz data formatted in a specific way:

	  <details>
	      <summary>
	          <object type="image/svg+xml" data="https://dot.skia.org/dot"></object>
	      </summary>
	      <pre>
	      graph {
              Hello -- World
	      }
          </pre>
	  </details>

The details/summary allows for showing the summary, the generated SVG,
while hiding the dot code in a way that makes it easy to inspect it.

We use an 'object' tag instead of an 'img' tag because that allows any
links in the SVG to be functional.

The 'pre' tag makes it easy to grab the dot code and also formats the dot
code nicely.