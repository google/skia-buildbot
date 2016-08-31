// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/logging"
	milocfg "github.com/luci/luci-go/milo/common/config"
	"github.com/luci/luci-go/server/router"
	"golang.org/x/net/context"
)

type Project struct {
	// The ID of the project, as per self defined.  This is not the luci-cfg
	// name.
	ID string `gae:"$id"`
	// The luci-cfg name of the project.
	Name string
	// The Project data in protobuf binary format.
	Data []byte `gae:",noindex"`
}

func UpdateHandler(ctx *router.Context) {
	c, h := ctx.Context, ctx.Writer
	err := update(c)
	if err != nil {
		logging.WithError(err).Errorf(c, "Update Handler encountered error")
		h.WriteHeader(500)
	}
	logging.Infof(c, "Successfully completed")
	h.WriteHeader(200)
}

// Update internal configuration based off luci-cfg.
// update updates Milo's configuration based off luci config.  This includes
// scanning through all project and extract all console configs.
func update(c context.Context) error {
	cfgName := info.Get(c).AppID() + ".cfg"
	cfgs, err := config.GetProjectConfigs(c, cfgName, false)
	if err != nil {
		logging.WithError(err).Errorf(c, "Encountered error while getting project config for %s", cfgName)
		return err
	}
	// A map of project ID to project.
	projects := map[string]*Project{}
	for _, cfg := range cfgs {
		pathParts := strings.SplitN(cfg.ConfigSet, "/", 2)
		if len(pathParts) != 2 {
			return fmt.Errorf("Invalid config path: %s", cfg.Path)
		}
		name := pathParts[1]
		proj := milocfg.Project{}
		logging.Infof(c, "Prossing %s", name)
		if err = proto.UnmarshalText(cfg.Content, &proj); err != nil {
			logging.WithError(err).Errorf(
				c, "Encountered error while processing %s.  Config:\n%s", name, cfg.Content)
			return err
		}
		if dup, ok := projects[proj.ID]; ok {
			return fmt.Errorf(
				"Duplicate project ID: %s. (%s and %s)", proj.ID, dup.Name, name)
		}
		p := &Project{
			ID:   proj.ID,
			Name: name,
		}
		projects[proj.ID] = p
		p.Data, err = proto.Marshal(&proj)
		if err != nil {
			return err
		}
	}

	// Now load all the data into the datastore.
	ds := datastore.Get(c)
	projs := make([]*Project, 0, len(projects))
	for _, proj := range projects {
		projs = append(projs, proj)
	}
	err = ds.Put(projs)
	if err != nil {
		return err
	}

	// Delete entries that no longer exist.
	q := datastore.NewQuery("Project").KeysOnly(true)
	allProjs := []Project{}
	ds.GetAll(q, &allProjs)
	toDelete := []Project{}
	for _, proj := range allProjs {
		if _, ok := projects[proj.ID]; !ok {
			toDelete = append(toDelete, proj)
		}
	}
	ds.Delete(toDelete)

	return nil
}

func GetAllProjects(c context.Context) ([]*milocfg.Project, error) {
	q := datastore.NewQuery("Project")
	q.Order("ID")
	ds := datastore.Get(c)
	ps := []*Project{}
	err := ds.GetAll(q, &ps)
	if err != nil {
		return nil, err
	}
	results := make([]*milocfg.Project, len(ps))
	for i, p := range ps {
		results[i] = &milocfg.Project{}
		if err := proto.Unmarshal(p.Data, results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func GetProject(c context.Context, projName string) (*milocfg.Project, error) {
	// Next, Try datastore
	ds := datastore.Get(c)
	p := Project{ID: projName}
	if err := ds.Get(&p); err != nil {
		return nil, err
	}
	mp := milocfg.Project{}
	if err := proto.Unmarshal(p.Data, &mp); err != nil {
		return nil, err
	}

	return &mp, nil
}

func GetConsole(c context.Context, projName, consoleName string) (*milocfg.Console, error) {
	p, err := GetProject(c, projName)
	if err != nil {
		return nil, err
	}
	for _, cs := range p.Consoles {
		if cs.Name == consoleName {
			return cs, nil
		}
	}
	return nil, fmt.Errorf("Console %s not found in project %s", consoleName, projName)
}
