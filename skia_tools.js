var skiaTools = {

PORT: "10117",
HOST: "http://70.32.156.51",

populateMenu: function(menuId, items) {
  var menu = document.getElementById(menuId);
  menu.options.length = 0;
  for (var i = 0; i < items.length; i++) {
    var item = items[i];
    var newOption = document.createElement("option");
    newOption.text = item;
    newOption.value = item;
    menu.options.add(newOption);
  }
},

loadDataForBuild: function(builder, build, loadUnfinished, loadUnknownRevs) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", this.HOST + ":" + this.PORT + "/json/builders/" + builder
               + "/builds/" + build + "/steps", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var buildData = eval("(" + request.responseText + ")");
  var buildDict = {"buildNum": build};
  var steps = [];
  var result = 0;
  var startTime = 0;
  var endTime = 0;
  var gotRevisionStr = "got_revision: ";
  for (var step in buildData) {
    var stepData = buildData[step];
    if (stepData["isStarted"] && !stepData["isFinished"] && !loadUnfinished) {
      // If the build isn't finished, ignore it
      return null;
    }
    if (!stepData["isStarted"]) {
      continue;
    }
    if (stepData["name"] == "Update") {
      if (stepData["isStarted"] && stepData["isFinished"] &&
          stepData["results"][0] == 0) {
        buildDict["revision"] = parseInt(stepData["text"][1].substring(
            gotRevisionStr.length));
      } else if (!loadUnknownRevs) {
        // If the Update step failed, we can't attach a revision, so we have
        // to ignore this build.
        console.log("Warning: Can't get a revision for build #" + build +
                    ". Skipping.");
        return null;
      }
    }
    var times = stepData["times"];
    var stepTime = times[1] - times[0];
    if (startTime == 0) {
      startTime = times[0];
    }
    endTime = times[1];
    try {
      stdout = stepData["logs"][0][1];
    } catch(e) {
      stdout = "None";
    }

    var stepDict = {};
    stepDict["name"] = stepData["name"];
    stepDict["time"] = stepTime;
    stepDict["stdio"] = stdout;
    stepDict["result"] = stepData["results"][0];
    steps.push(stepDict);

    if (stepDict["result"] != 0) {
      result++;
    }
  }
  if (buildDict["revision"] == undefined) {
    console.log("Warning: could not find a revision for build #" + build);
  }
  buildDict["result"] = result;
  buildDict["steps"] = steps;
  buildDict["startTime"] = startTime;
  buildDict["endTime"] = endTime;
  buildDict["time"] = endTime - startTime;
  return buildDict;
},

loadDataForBuilder: function(builderName, builderDict, numBuilds, revList,
                            revData) {
  var builds = builderDict['cachedBuilds'];
  for (i = 1; i <= numBuilds; i++) {
    if (builds.length - i < 0) { continue; }
    var buildData = this.loadDataForBuild(builderName,
                                          builds[builds.length - i],
                                          false, false);
    if (null == buildData) { continue; }
    var rev = buildData["revision"];
    if (rev == undefined) { continue; }
    if (revData[rev] == undefined) {
      revData[rev] = {}
      revList.push(rev);
    }
    revData[rev][builderName] = buildData;
  }
},

loadDataForSlave: function(slaveDict, rangeMin, currentTime) {
  var builders = slaveDict["builders"];
  slaveDict["rangeMin"] = Number.POSITIVE_INFINITY;
  slaveDict["rangeMax"] = Number.NEGATIVE_INFINITY;
  slaveDict["allBuilds"] = [];
  for (var builder in builders) {
    var builds = builders[builder];
    for (var i = 0; i < builds.length; i++) {
      var build = builds[i];
      var buildData = this.loadDataForBuild(builder, build, true, true);
      if (buildData) {
        slaveDict["allBuilds"].push([builder, buildData["buildNum"],
            buildData["startTime"], buildData["endTime"]]);
        if (buildData["startTime"] < slaveDict["rangeMin"]) {
          slaveDict["rangeMin"] = buildData["startTime"];
        }
        if (buildData["endTime"] > slaveDict["rangeMax"]) {
          slaveDict["rangeMax"] = buildData["endTime"];
        }
        if (buildData["endTime"] < rangeMin ||
            buildData["startTime"] < rangeMin) {
          break;
        }
      }
    }
  }
  var runningBuilds = slaveDict["runningBuilds"];
  for (var i = 0; i < runningBuilds.length; i++) {
    var build = runningBuilds[i];
    slaveDict["allBuilds"].push([build["builderName"], build["number"],
                                 build["times"][0], currentTime + 1]);
  }
  slaveDict["allBuilds"].sort(function(a, b) {
    return a[2] - b[2];
  });
},

loadBuilders: function(builderList) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", this.HOST + ":" + this.PORT + "/json/builders", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var buildersDict = eval("(" + request.responseText + ")");
  for (var builder in buildersDict) {
    builderList.push(builder);
  }
  return buildersDict;
},

loadSlaves: function(slaveList) {
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", this.HOST + ":" + this.PORT + "/json/slaves", false);
  request.send(null);
  // We *should* use a JSON parser, but since we trust the buildbot master
  // server, we allow this unsafe call 
  var slavesDict = eval("(" + request.responseText + ")");
  for (var slave in slavesDict) {
    slaveList.push(slave);
  }
  return slavesDict;
},

};