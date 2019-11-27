# Overview

This is an overview of how Skia is built and tested and the relationships
between the various services we use.


## Life of a +1

What happens when a user submits a CL to the Commit Queue (CQ):

<details>
  <summary>
    <img src="https://dot.skia.org/dot?lifeof">
  </summary>
  <pre>
digraph {
  rankdir=LR;
  node [shape = box, fixedsize = true, width=2, height=0.6];

  IN, OUT [style = invis];

  // The top level input and output flows, a +2 goes in and eventually an email is sent
  // on success or failure.
  IN -> Gerrit [ label="+2" ];
  Gerrit -> OUT [ label="email" ];

  // Requests flowing down to Swarming to do the building, testing, and uploading.
  Gerrit -> CQ [ label="CL (poll)" ];
  LUCIConfig -> CQ [ label="TryJobs\n(poll)" ];
  CQ -> BuildBucket [ label="create(CL, TryJob)\n(API)" ];
  BuildBucket -> TaskScheduler [ label="CL+TryJob\n(poll)" ];
  TaskScheduler -> Swarming [ label="TaskInfo\nCIPD List + Isolate List" ];
  TaskScheduler -> Isolate [ label="HEAD+CL" ];
  CIPD -> Swarming [ label="(pull)" ];
  Isolate -> Swarming [ label="(pull)" ];

  // Propagating the signal that the job is done.
  Swarming -> TaskScheduler [ label="TaskResult\nPubSub+poll", style=dotted ];
  TaskScheduler -> BuildBucket [ label="Done", style=dotted ];
  BuildBucket -> CQ [ label="Done", style=dotted ];
  CQ -> Gerrit [ label="Done", style=dotted ];
}
   </pre>
</details>