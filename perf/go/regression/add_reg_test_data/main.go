package main

import (
	"encoding/json"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	idb "go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/regression"
)

const (
	clSource = `{
 "centroid": [
  -1.0758549,
  -1.077101,
  -1.0868026,
  -1.0686587,
  -1.0927571,
  0.8945945,
  0.8768508,
  0.89801604,
  0.90060735,
  0.92201734,
  0.90909344
 ],
 "keys": [
  ",arch=x86,bench_type=playback,clip=0_0_1000_1000,compiler=Clang,config=8888,cpu_or_gpu=CPU,cpu_or_gpu_value=Moorefield,extra_config=GN_Android,model=NexusPlayer,multi_picture_draw=false,name=desk_samoasvg.skp,os=Android,scale=1.1,source_type=skp,sub_result=min_ms,test=desk_samoasvg.skp_1.1_1000_1000,",
  ",arch=x86_64,bench_type=playback,clip=0_0_1000_1000,compiler=MSVC,config=gpu,cpu_or_gpu=GPU,cpu_or_gpu_value=GTX960,extra_config=ANGLE,model=ShuttleC,multi_picture_draw=true,name=desk_samoasvg.skp,os=Win,scale=1,source_type=skp,sub_result=min_ms,test=desk_samoasvg.skp_1_mpd_1000_1000,",
  ",arch=x86_64,bench_type=playback,clip=0_0_1000_1000,compiler=MSVC,config=gpu,cpu_or_gpu=GPU,cpu_or_gpu_value=GTX960,extra_config=ANGLE,model=ShuttleC,multi_picture_draw=true,name=desk_mapsvg.skp,os=Win,scale=1,source_type=skp,sub_result=min_ms,test=desk_mapsvg.skp_1_mpd_1000_1000,",
  ",arch=x86_64,bench_type=playback,clip=0_0_1000_1000,compiler=MSVC,config=nvprdit16,cpu_or_gpu=GPU,cpu_or_gpu_value=GTX960,model=ShuttleC,multi_picture_draw=true,name=desk_wikipedia.skp,os=Win,scale=1,source_type=skp,sub_result=min_ms,test=desk_wikipedia.skp_1_mpd_1000_1000,",
  ",arch=x86,bench_type=playback,clip=0_0_1000_1000,compiler=Clang,config=srgb,cpu_or_gpu=CPU,cpu_or_gpu_value=Moorefield,extra_config=GN_Android,model=NexusPlayer,multi_picture_draw=false,name=desk_mapsvg.skp,os=Android,scale=1.1,source_type=skp,sub_result=min_ms,test=desk_mapsvg.skp_1.1_1000_1000,"
 ],
 "param_summaries": {
  "arch": [
   {
    "value": "x86_64",
    "weight": 20
   },
   {
    "value": "arm64",
    "weight": 15
   },
   {
    "value": "arm",
    "weight": 13
   },
   {
    "value": "x86",
    "weight": 12
   }
  ],
  "bench_type": [
   {
    "value": "playback",
    "weight": 25
   },
   {
    "value": "piping",
    "weight": 12
   },
   {
    "value": "recording",
    "weight": 12
   }
  ],
  "clip": [
   {
    "value": "0_0_1000_1000",
    "weight": 26
   }
  ],
  "compiler": [
   {
    "value": "Clang",
    "weight": 19
   },
   {
    "value": "MSVC",
    "weight": 17
   },
   {
    "value": "GCC",
    "weight": 13
   }
  ]
 },
 "step_fit": {
  "least_squares": 0.003520409,
  "turning_point": 5,
  "step_size": -1.9804314,
  "regression": -562.5572,
  "status": "Low"
 },
 "step_point": {
  "source": "master",
  "offset": 25004,
  "timestamp": 1478423103
 },
 "num": 2291,
 "ID": -1
}`

	dfSource = `{
 "dataframe": {
  "traceset": {},
  "header": [
   {
    "source": "master",
    "offset": 24999,
    "timestamp": 1478293861
   },
   {
    "source": "master",
    "offset": 25000,
    "timestamp": 1478294123
   },
   {
    "source": "master",
    "offset": 25001,
    "timestamp": 1478297361
   },
   {
    "source": "master",
    "offset": 25002,
    "timestamp": 1478352862
   },
   {
    "source": "master",
    "offset": 25003,
    "timestamp": 1478377727
   },
   {
    "source": "master",
    "offset": 25004,
    "timestamp": 1478423103
   },
   {
    "source": "master",
    "offset": 25005,
    "timestamp": 1478436410
   },
   {
    "source": "master",
    "offset": 25006,
    "timestamp": 1478436442
   },
   {
    "source": "master",
    "offset": 25007,
    "timestamp": 1478447077
   },
   {
    "source": "master",
    "offset": 25008,
    "timestamp": 1478495222
   },
   {
    "source": "master",
    "offset": 25009,
    "timestamp": 1478524168
   }
  ],
  "paramset": {
   "arch": [
    "arm",
    "arm64",
    "x86",
    "x86_64"
   ],
   "bench_type": [
    "piping",
    "playback",
    "recording"
   ],
   "clip": [
    "0_0_1000_1000"
   ],
   "compiler": [
    "Clang",
    "GCC",
    "MSVC"
   ],
   "os": [
    "Android",
    "Mac",
    "Ubuntu",
    "Win",
    "Win10",
    "Win8"
   ],
   "scale": [
    "1",
    "1.1"
   ],
   "source_type": [
    "skp"
   ],
   "sub_result": [
    "min_ms"
   ]
  },
  "skip": 0
 },
 "ticks": [
  [ 0, "Fri" ],
  [ 2.5, "Sat" ],
  [ 4.5, "Sun" ],
  [ 8.5, "Mon" ]
 ],
 "skps": [ 5 ],
 "msg": ""
}`
)

func main() {
	defer common.LogPanic()
	// Setup DB flags.
	dbConf := idb.DBConfigFromFlags()

	common.Init()

	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	store := regression.NewStore()
	git, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia", "/usr/local/google/home/jcgregorio/projects/skia_for_perf/", false)
	if err != nil {
		glog.Fatal(err)
	}
	offset, err := git.IndexOf("301dfc0bba37bbda8a269b066d3fd0e9b16b7cd0")
	if err != nil {
		glog.Fatal(err)
	}
	cidl := cid.New(git, nil)
	c := &cid.CommitID{
		Source: "master",
		Offset: offset,
	}
	cids, err := cidl.Lookup([]*cid.CommitID{c})
	if err != nil {
		glog.Fatal(err)
	}

	df := &dataframe.FrameResponse{}
	err = json.Unmarshal([]byte(dfSource), df)
	if err != nil {
		glog.Fatal(err)
	}
	cl := &clustering2.ClusterSummary{}
	err = json.Unmarshal([]byte(clSource), cl)
	if err != nil {
		glog.Fatal(err)
	}
	err = store.SetHigh(cids[0], "source_type=skp&sub_result=min_ms", df, cl)
	if err != nil {
		glog.Fatal(err)
	}
}
