suggester
=========

A system that suggests which trybots should be run for a given CL.

https://skia-review.googlesource.com/changes/81121/revisions/8/files/

)]}'
{
  "/COMMIT_MSG": {
    "status": "A",
    "lines_inserted": 10,
    "size_delta": 353,
    "size": 353
  },
  "BUILD.gn": {
    "lines_inserted": 20,
    "lines_deleted": 5,
    "size_delta": 531,
    "size": 50072
  },
  "include/gpu/vk/GrVkDefines.h": {
    "lines_inserted": 28,
    "lines_deleted": 21,
    "size_delta": 383,
    "size": 1615
  },
  "tools/gpu/vk/GrVulkanDefines.h": {
    "status": "A",
    "lines_inserted": 33,
    "size_delta": 861,
    "size": 861
  }
}

Run ./bin/try --list to get a list of valid trybots.


resp, err := swarmApi.ListTasks(time.Time{}, time.Time{}, []string{}, "completed_failure")
	resp[0].TaskResult.Tags


    // Result from a trybot.

    Tags map[
      sk_issue_server:https://skia-review.googlesource.com
      sk_issue:82041
      sk_patchset:1
      sk_name:Test-Android-Clang-Nexus7-CPU-Tegra3-arm-Debug-All-Android
      sk_repo:https://skia.googlesource.com/skia.git

      sk_retry_of:
      device_type:grouper
      sk_dim_pool:Skia
      sk_attempt:0
      sk_parent_task_id:20171207T175742.807991418Z_00000000003807fe
      user:skiabot@google.com
      pool:Skia
      priority:80
      sk_dim_device_type:grouper
      sk_forced_job_id:
      source_revision:1cfb6bc9b63e9840d198a1ea8b1a20da2bfde818
      service_account:none
      sk_id:20171207T180312.145288450Z_000000000038087b
      sk_priority:0.800000
      sk_revision:1cfb6bc9b63e9840d198a1ea8b1a20da2bfde818
      device_os:LMY47V_1836172
      luci_project:skia
      os:Android
      sk_dim_device_os:LMY47V_1836172
      sk_dim_python:2.7.9
      python:2.7.9
      sk_dim_os:Android
      source_repo:https://skia.googlesource.com/skia.git/+/%s]

    // Result from waterfall

    Tags map[
      sk_revision:1cfb6bc9b63e9840d198a1ea8b1a20da2bfde818
      sk_name:Test-Win10-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan
      sk_repo:https://skia.googlesource.com/skia.git

      gpu:10de:1cb3-22.21.13.8205
      sk_dim_gpu:10de:1cb3-22.21.13.8205
      service_account:none
      sk_dim_os:Windows-10-15063
      sk_dim_pool:Skia
      sk_forced_job_id:
      source_repo:https://skia.googlesource.com/skia.git/+/%s
      pool:Skia
      priority:80
      sk_parent_task_id:20171207T173736.503845536Z_0000000000380503
      sk_priority:0.800000
      sk_retry_of:
      source_revision:1cfb6bc9b63e9840d198a1ea8b1a20da2bfde818
      os:Windows-10-15063
      sk_id:20171207T180305.395050506Z_000000000038086a
      user:skiabot@google.com
      luci_project:skia
      sk_attempt:0]
