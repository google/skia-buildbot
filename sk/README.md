# Skia SK Tool

**Fetching**

The `sk` program is fetched into the Skia repo by:

1. `cd bin`
2. `./fetch-sk`

## `release-branch`

The release-branch subcommand automates several tasks done on each Skia
release. It is safe to run as it only creates Gerrit CLs to be reviewed and
makes no permanent changes. It does the following:

1. Creates a CL to update the Skia milestone in `include/core/SkMilestone.h` in
   the **main** branch.
   Example: http://review.skia.org/661110
2. Creates a CL to update [supported-branches.json](https://skia.googlesource.com/skia/+/refs/heads/infra/config/supported-branches.json)
   in the **infra/config** branch to edit the commit queue config to add the new
   branch and remove the outgoing (e.g. M-4).
   Example: http://review.skia.org/661111
3. Creates a CL to filter out unsupported CQ try jobs on the **new milestone**
   branch.
   Example: http://review.skia.org/661360
4. Creates a CL to remove the M-4 branch from the CQ in the **M-4** branch.
   Example: http://review.skia.org/661361
5. Creates a CL to merge individual release notes into the top level
   RELEASE_NOTES.md file in the **new milestone** branch.
6. Creates a CL to cherry-pick the RELEASE_NOTES.md merge into the **main
   branch**.

### Running `release-branch`

Run as so:

```sh
./sk release-branch <release branch name>
```

For example:

```sh
./sk release-branch chrome/m113
```

### Testing `release-branch`

release-branch works by creating CLs – which eventually merge into their target
branches. Once this is done, running a second time will fail as the changes
have already been made and `release-branch` treats empty CLs as an error. To
enable testing run with the `allow-empty` flag:

```sh
./sk release-branch --allow-empty chrome/m113
```

After running, if all goes well, new CLs will be in the outgoing section of
https://skia-review.googlesource.com **in your name**. Some steps are smart
enough to know that the change is already made and will not produce a new CL.
As this is a test, they can be deleted once verified.

## `sk` Deployment

Changes to `sk` are automatically pulled into Skia via an autoroller.
Skia's `fetch-sk` retrieves the `sk` version from the DEPS file and pulls that
version down from CIPD for the correct platform.

Avoid changing `sk` in the final days before a Skia branch without first
coordinating with the Skia release manager. Skia is branched alongside Chrome,
and the Chrome branch schedule can be found at
https://chromiumdash.appspot.com/schedule.
