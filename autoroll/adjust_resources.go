package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	config    = flag.String("config", "", "File or directory to change.")
	container = flag.String("container", "autoroll-be.*", "Container name or regular expression")
)

func main() {
	common.Init()

	if *config == "" {
		log.Fatal("--config is required")
	}
	if *container == "" {
		log.Fatal("--container is required")
	}
	/*re, err := regexp.Compile(*container)
	if err != nil {
		log.Fatal(err)
	}*/
	st, err := os.Stat(*config)
	if err != nil {
		log.Fatal(err)
	}
	var configFiles map[string]string
	if st.IsDir() {
		ls, err := ioutil.ReadDir(*config)
		if err != nil {
			sklog.Fatal(err)
		}
		configFiles = make(map[string]string, len(ls))
		for _, fi := range ls {
			f := filepath.Join(*config, fi.Name())
			b, err := ioutil.ReadFile(f)
			if err != nil {
				log.Fatal(err)
			}
			configFiles[f] = string(b)
		}
	} else {
		b, err := ioutil.ReadFile(*config)
		if err != nil {
			log.Fatal(err)
		}
		configFiles = map[string]string{*config: string(b)}
	}

	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		log.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	cpu := get(c, fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{container_name!="POD",container_name=~"%s"}[7d])) by (container_name)`, *container))
	mem := get(c, fmt.Sprintf(`max_over_time(container_memory_usage_bytes{container_name!="POD",container_name=~"%s"}[7d]) / 1024 / 1024`, *container))
	pts := make(map[string]*point, len(cpu))
	for k, v := range cpu {
		pts[k] = &point{
			Label:  k,
			UseCPU: v,
		}
	}
	for k, v := range mem {
		if _, ok := pts[k]; !ok {
			pts[k] = &point{
				Label:  k,
				UseCPU: -1,
			}
		}
		pts[k].UseMem = v
	}
	ptSlice := make([]*point, 0, len(pts))
	for _, pt := range pts {
		ptSlice = append(ptSlice, pt)
	}
	sort.Sort(points(ptSlice))
	for _, pt := range ptSlice {
		pt.ReqCPU = math.Ceil(pt.UseCPU*100.0) / 100.0 * 1.2
		pt.ReqMem = math.Ceil(pt.UseMem/100.0) * 100.0 * 1.2
		fmt.Println(fmt.Sprintf("%s:\t%.4f -> %.4f CPU\t%.4f -> %.4f MiB Mem", pt.Label, pt.UseCPU, pt.ReqCPU, pt.UseMem, pt.ReqMem))
	}

	// Write the updated configs.
	nameRe, err := regexp.Compile(`roller[N|n]ame"?:\s*"?([a-zA-Z0-9_-]+)"?,?`)
	if err != nil {
		log.Fatal(err)
	}
	cpuRe, err := regexp.Compile(`"?cpu"?:\s*"(\w+)"`)
	if err != nil {
		log.Fatal(err)
	}
	memRe, err := regexp.Compile(`"?memory"?:\s*"(\w+)"`)
	if err != nil {
		log.Fatal(err)
	}
	for f, cfg := range configFiles {
		container := ""
		m := nameRe.FindStringSubmatch(cfg)
		if len(m) == 2 {
			container = "autoroll-be-" + m[1]
		}
		if container == "" {
			log.Fatalf("No container found for %s", f)
		}
		pt, ok := pts[container]
		if !ok {
			log.Fatalf("No metrics found for %s", container)
		}
		cpuMatch := cpuRe.FindStringSubmatch(cfg)
		if len(cpuMatch) == 2 {
			repl := strings.Replace(cpuMatch[0], cpuMatch[1], fmt.Sprintf("%dm", int64(pt.ReqCPU*1000.0)), 1)
			cfg = cpuRe.ReplaceAllString(cfg, repl)
		}
		memMatch := memRe.FindStringSubmatch(cfg)
		if len(memMatch) == 2 {
			repl := strings.Replace(memMatch[0], memMatch[1], fmt.Sprintf("%dMi", int64(pt.ReqMem)), 1)
			cfg = memRe.ReplaceAllString(cfg, repl)
		}
		if err := ioutil.WriteFile(f, []byte(cfg), os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
}

type point struct {
	Label  string
	UseCPU float64
	UseMem float64
	ReqCPU float64
	ReqMem float64
}

type points []*point

func (p points) Len() int           { return len(p) }
func (p points) Less(a, b int) bool { return p[a].Label < p[b].Label }
func (p points) Swap(a, b int)      { p[a], p[b] = p[b], p[a] }

func get(c *http.Client, query string) map[string]float64 {
	resp, err := c.Get(fmt.Sprintf("https://prom2.skia.org/api/v1/query?query=%s", url.QueryEscape(query)))
	if err != nil {
		log.Fatal(err)
	}
	defer util.Close(resp.Body)

	var results struct {
		Data struct {
			Result []struct {
				Metric struct {
					Container string `json:"container_name"`
				} `json:"metric"`
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Fatal(err)
	}

	rv := map[string]float64{}
	for _, res := range results.Data.Result {
		v, err := strconv.ParseFloat(res.Value[1].(string), 64)
		if err != nil {
			log.Fatal(err)
		}
		rv[res.Metric.Container] = v
	}
	return rv
}
