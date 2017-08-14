Cluster Telemetry Pixel Diff Server
====================================================

* Processes image diffs for the Cluster Telemetry benchmark found here:
https://ct.skia.org/pixel_diff/.
* Running this benchmark will take screenshots of the top 10k websites with and
without the specified patch, and store these images along with metadata, in
Google Storage.
* This server continuously detects new runs of the benchmark and parses the
corresponding metadata in order to organize the image diffing between the
no-patch and with-patch screenshots for each site.
* An instance of Skia Gold's DiffStore is then utilized to calculate diff
metrics and create the diff image for each pair of screenshots.
* Server's UI allows users to select a Cluster Telemetry Pixel Diff run, and
view all the screenshots and diff results corresponding to that run. Link:
https://ctpixeldiff.skia.org/
* Disclaimer: diff results may contain NSFW images.
