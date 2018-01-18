DESIGN
======

Build a model that predicts which bots a CL should be run against
to find failures.

1. Find all failures for the last 3 months in swarming, which includes all
   trybots and jobs on the waterfall.
2. Find the list of changed files in each of those failures and record both
   the bot name and the files.
3. As a first pass model, just add up the number of times a bot has failed
   when each file has appeared in a CL. To get a prediction for a CL, just
   get a prediction for each file and then add up all the bot counts.

For now try jobs are ingoreed as that data is too noisy and only jobs that
appear on the waterfall are used.

In conjunction with this analysis the application also tracks when bots are
flagged as flaky, to exclude them from the prediction model.
