/**
 * @type {import("puppeteer").Configuration}
 */
module.exports = {
  // By default, Puppeteer downloads Chrome to ~/.cache/puppeteer as a post-install step (see
  // https://pptr.dev/guides/configuration). This non-hermetic behavior does not play well with
  // Bazel. Instead, we disable it, and we hermetically download Chrome ourselves via the
  skipDownload: true,
};
