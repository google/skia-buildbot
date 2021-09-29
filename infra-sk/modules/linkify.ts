/** @module infra-sk/modules/linkify */

/**
 * Given a string (usually given by an untrustworthy user), this returns a lit-html Part
 * which contains the string contents with the following "improvements":
 *   * Any HTML is escaped ("<" and ">" become "&lt;", "&gt;", etc.).
 *   * URLs are wrapped in anchor tags.
 *   * Line breaks are replaced with <br>.
 *   * Bug shorthand (e.g. skia:1234) is wrapped in anchor tags.
 *
 * Example usage:
 *
 *   html`user input: ${escapeAndLinkify(untrustedUserInput)}`;
 */
export function escapeAndLinkify(s: string): HTMLDivElement | string {
  if (!s) {
    return '';
  }
  // sanitize the incoming string, so we aren't vulnerable to XSS.
  s = s.replace(/</g, '&lt');
  s = s.replace(/>/g, '&gt');

  // Replace http://... with actual links
  const sub = '<a href="$&" target=_blank rel=noopener>$&</a>';
  s = s.replace(/https?:\/\/\S+/g, sub);
  // Replace newlines with <br>
  s = s.replace(/(?:\r\n|\n|\r)/g, '<br>');

  // Replace things like skia:1234 with actual links
  for (const project of supportedIssueTrackers) {
    const foundBugs = s.match(project.re);
    if (!foundBugs) {
      continue;
    }
    for (const foundBug of foundBugs) {
      const bugNumber = foundBug.split(':')[1];
      s = s.replace(foundBug, `<a href="${project.url + bugNumber}"
                                  target=_blank rel=noopener>${foundBug}</a>`);
    }
  }
  const div = document.createElement('div');
  div.innerHTML = s;
  return div;
}

const supportedIssueTrackers = [
  {
    re: /chromium:[0-9]+/g,
    url: 'http://crbug.com/',
  }, {
    re: /skia:[0-9]+/g,
    url: 'http://skbug.com/',
  }];
