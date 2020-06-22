export const chromiumPatchResult = {
  catapult_patch: '',
  chromium_patch: "\n\ndiff --git a/DEPS b/DEPS\nindex 849ae22..ee07579 100644\n--- a/DEPS\n+++ b/DEPS\n@@ -178,7 +178,7 @@\n   # Three lines of non-changing comments so that\n   # the commit queue can handle CLs rolling Skia\n   # and whatever else without interference from each other.\n-  'skia_revision': 'cc7ec24ca824ca13d5a8a8e562fcec695ae54390',\n+  'skia_revision': '1dbc3b533962b0ae803a2a5ee89f61146228d73b',\n   # Three lines of non-changing comments so that\n   # the commit queue can handle CLs rolling V8\n   # and whatever else without interference from each other.\n",
  cl: '123',
  modified: '20200529230004',
  skia_patch: '',
  subject: 'Roll Skia from cc7ec24ca824 to \n\n\n1dbc3b533962 (3 revisions)',
  url: 'https://chromium-review.googlesource.com/c/2222715/3',
  v8_patch: '',
};

// A bloated result used to mock any of the valid patch types.
export const anyPatchResult = {
  catapult_patch: 'imagine a git diff for a catapult patch',
  chromium_patch: 'imagine a git diff for a chromium patch',
  chromium_patch_base_build: 'imagine a git diff for a chromium_base_build patch',
  cl: '123',
  modified: '20200529230004',
  skia_patch: 'imagine a git diff for a skia patch',
  subject: 'Roll Skia from cc7ec24ca824 to \n\n\n1dbc3b533962 (3 revisions)',
  url: 'https://chromium-review.googlesource.com/c/2222715/3',
  v8_patch: 'imagine a git diff for a V8 patch',
};
