Design of the New Rebaseline Server
===================================

Initial key features:

* Independent of Skia's specific tests (i.e. DM vs GM), but abstract enough
  to handle different testing scenarios (SKPs, cluster telemetry, Blink, Android and beyond).
* Allow posting of DM output for fast comparison (to be called from Buildbot).
* Frontend UI allow to maintain image baselines.
* Instead of maintaining expectation on a per builder basis we will maintain
  a set of expectations for each test.


JSON Input Example
------------------

{
  "gitHash": "d1830323662ae8ae06908b97f15180fd25808894",
  "key": {
    "arch": "x86",
    "gpu": "GTX660",
    "os": "Ubuntu12",
    "model": "ShuttleA",
  },
  "results": [
     {
         "key" : {
            "config" : "565",
            "name" : "verttext"
         },
         "md5" : "6251defe4bf6f79efb9e7f3f93c718e2",
         "options" : {
            "source_type" : "GM"
         }
      },
      {
         "key" : {
            "config" : "8888",
            "name" : "verttext2"
         },
         "md5" : "8555ccf1f3d0d11d09837733b213f86f",
         "options" : {
            "source_type" : "GM"
         }
      },
      ...
   ]
}
