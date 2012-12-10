var internalport = "10115";
var externalport = "10117";
var port = externalport;
var host = "http://70.32.156.51";

function populateMenu(menu_id, items) {
  var menu = document.getElementById(menu_id);
  menu.options.length = 0;
  for (var i = 0; i < items.length; i++) {
    var item = items[i];
    var new_option = document.createElement("option");
    new_option.text = item;
    new_option.value = item;
    menu.options.add(new_option);
  }
}

function loadDataForBuild(builder, build, load_unfinished, load_unknown_revs) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", host + ":" + port + "/json/builders/" + builder +
               "/builds/" + build + "/steps", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var build_data = eval("(" + request.responseText + ")");
  var build_dict = {"build_num": build};
  var steps = [];
  var result = 0;
  var start_time = 0;
  var end_time = 0;
  var got_revision_str = "got_revision: ";
  for (var step in build_data) {
    var step_data = build_data[step];
    if (step_data["isStarted"] && !step_data["isFinished"] &&
        !load_unfinished) {
      // If the build isn't finished, ignore it
      return null;
    }
    if (!step_data["isStarted"]) {
      continue;
    }
    if (step_data["name"] == "Update") {
      if (!(step_data["isStarted"] && step_data["isFinished"] &&
            step_data["results"][0] == 0) && !load_unknown_revs) {
        // If the Update step failed, we can't attach a revision, so we have
        // to ignore this build.
        console.log("Warning: Can't get a revision for build #" + build +
                    ". Skipping.");
        return null;
      }
      build_dict["revision"] = parseInt(step_data["text"][1].substring(
          got_revision_str.length));
    }
    var times = step_data["times"];
    var step_time = times[1] - times[0];
    if (start_time == 0) {
      start_time = times[0];
    }
    end_time = times[1];
    try {
      stdout = step_data["logs"][0][1];
    } catch(e) {
      stdout = "None";
    }

    var step_dict = {};
    step_dict["name"] = step_data["name"];
    step_dict["time"] = step_time;
    step_dict["stdio"] = stdout;
    step_dict["result"] = step_data["results"][0];
    steps.push(step_dict);

    if (step_dict["result"] != 0) {
      result++;
    }
  }
  if (build_dict["revision"] == undefined) {
    console.log("Warning: could not find a revision for build #" + build);
  }
  build_dict["result"] = result;
  build_dict["steps"] = steps;
  build_dict["start_time"] = start_time;
  build_dict["end_time"] = end_time;
  build_dict["time"] = end_time - start_time;
  return build_dict;
}

function loadDataForBuilder(builder_name, builder_dict, num_builds, rev_list,
                            rev_data) {
  var builds = builder_dict['cachedBuilds'];
  for (i = 1; i <= num_builds; i++) {
    if (builds.length - i < 0) { continue; }
    var build_data = loadDataForBuild(builder_name, builds[builds.length - i],
                                      false, false);
    if (null == build_data) { continue; }
    var rev = build_data["revision"];
    if (rev == undefined) { continue; }
    if (rev_data[rev] == undefined) {
      rev_data[rev] = {}
      rev_list.push(rev);
    }
    rev_data[rev][builder_name] = build_data;
  }
}

function loadDataForSlave(slave_name, slave_dict, range_min, current_time) {
  var builders = slave_dict["builders"];
  slave_dict["range_min"] = Number.POSITIVE_INFINITY;
  slave_dict["range_max"] = Number.NEGATIVE_INFINITY;
  slave_dict["all_builds"] = [];
  for (var builder in builders) {
    var builds = builders[builder];
    for (var i = 0; i < builds.length; i++) {
      var build = builds[i];
      var build_data = loadDataForBuild(builder, build, true, true);
      if (build_data) {
        slave_dict["all_builds"].push([builder, build_data["build_num"],
            build_data["start_time"], build_data["end_time"]]);
        if (build_data["start_time"] < slave_dict["range_min"]) {
          slave_dict["range_min"] = build_data["start_time"];
        }
        if (build_data["end_time"] > slave_dict["range_max"]) {
          slave_dict["range_max"] = build_data["end_time"];
        }
        if (build_data["end_time"] < range_min ||
            build_data["start_time"] < range_min) {
          break;
        }
      }
    }
  }
  var current_builds = slave_dict["runningBuilds"];
  for (var i = 0; i < current_builds.length; i++) {
    var build = current_builds[i];
    slave_dict["all_builds"].push([build["builderName"], build["number"],
                                   build["times"][0], current_time + 1]);
  }
  slave_dict["all_builds"].sort(function(a, b) {
    return a[2] - b[2];
  });
}

function loadBuilders(builder_list) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", host + ":" + port + "/json/builders", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var builders_dict = eval("(" + request.responseText + ")");
  for (var builder in builders_dict) {
    builder_list.push(builder);
  }
  return builders_dict;
}

function loadSlaves(slave_list) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", host + ":" + port + "/json/slaves", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var slaves_dict = eval("(" + request.responseText + ")");
  for (var slave in slaves_dict) {
    slave_list.push(slave);
  }
  return slaves_dict;
}