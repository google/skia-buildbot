// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luci/gae/service/datastore"
)

const flipMask uint32 = 0xFFFFFFFF

var _ datastore.PropertyConverter = (*Attempt_ID)(nil)

// NewQuestID is a shorthand to New a new *Quest_ID
func NewQuestID(qst string) *Quest_ID {
	return &Quest_ID{qst}
}

// NewAttemptID is a shorthand to New a new *Attempt_ID
func NewAttemptID(qst string, aid uint32) *Attempt_ID {
	return &Attempt_ID{qst, aid}
}

// NewExecutionID is a shorthand to New a new *Execution_ID
func NewExecutionID(qst string, aid, eid uint32) *Execution_ID {
	return &Execution_ID{qst, aid, eid}
}

// ToProperty implements datastore.PropertyConverter for the purpose of
// embedding this Attempt_ID as the ID of a luci/gae compatible datastore
// object. The numerical id field is stored as an inverted, hex-encoded string,
// so that Attempt_ID{"quest", 1} would encode as "quest|fffffffe". This is done
// so that the __key__ ordering in the dm application prefers to order the most
// recent attempts first.
//
// The go representation will always have the normal non-flipped numerical id.
func (a *Attempt_ID) ToProperty() (datastore.Property, error) {
	return datastore.MkPropertyNI(a.DMEncoded()), nil
}

// FromProperty implements datastore.PropertyConverter
func (a *Attempt_ID) FromProperty(p datastore.Property) error {
	if p.Type() != datastore.PTString {
		return fmt.Errorf("wrong type for property: %s", p.Type())
	}
	return a.SetDMEncoded(p.Value().(string))
}

// DMEncoded returns the encoded string id for this Attempt. Numeric values are
// inverted if flip is true.
func (a *Attempt_ID) DMEncoded() string {
	return fmt.Sprintf("%s|%08x", a.Quest, flipMask^a.Id)
}

// SetDMEncoded decodes val into this Attempt_ID, returning an error if
// there's a problem. Numeric values are inverted if flip is true.
func (a *Attempt_ID) SetDMEncoded(val string) error {
	toks := strings.SplitN(val, "|", 2)
	if len(toks) != 2 {
		return fmt.Errorf("unable to parse Attempt id: %q", val)
	}
	an, err := strconv.ParseUint(toks[1], 16, 32)
	if err != nil {
		return err
	}

	a.Quest = toks[0]
	a.Id = flipMask ^ uint32(an)
	return nil
}

// GetQuest gets the specified quest from GraphData, if it's already there. If
// it's not, then a new Quest will be created, added, and returned.
//
// If the Quests map is uninitialized, this will initialize it.
func (g *GraphData) GetQuest(qid string) (*Quest, bool) {
	cur, ok := g.Quests[qid]
	if !ok {
		cur = &Quest{
			Id:       NewQuestID(qid),
			Attempts: map[uint32]*Attempt{},
		}
		if g.Quests == nil {
			g.Quests = map[string]*Quest{}
		}
		g.Quests[qid] = cur
	}
	return cur, ok
}

// NewQuestDesc is a shorthand method for building a new *Quest_Desc.
func NewQuestDesc(cfg string, params, distParams string, meta *Quest_Desc_Meta) *Quest_Desc {
	return &Quest_Desc{cfg, params, distParams, meta}
}

// NewTemplateSpec is a shorthand method for building a new *Quest_TemplateSpec.
func NewTemplateSpec(project, ref, version, name string) *Quest_TemplateSpec {
	return &Quest_TemplateSpec{project, ref, version, name}
}

// Equals returns true iff this Quest_TemplateSpec matches all of the fields of
// the `o` Quest_TemplateSpec.
func (t *Quest_TemplateSpec) Equals(o *Quest_TemplateSpec) bool {
	return *t == *o
}

// Less returns true iff this Quest_TemplateSpec sorts before the other one.
func (t *Quest_TemplateSpec) Less(o *Quest_TemplateSpec) bool {
	cmp := []struct{ a, b string }{
		{t.Project, o.Project},
		{t.Ref, o.Ref},
		{t.Version, o.Version},
		{t.Name, o.Name},
	}
	for _, c := range cmp {
		if c.a < c.b {
			return true
		} else if c.a > c.b {
			return false
		}
	}
	return false
}

// QuestTemplateSpecs is a sortable slice of *Quest_TemplateSpec.
type QuestTemplateSpecs []*Quest_TemplateSpec

func (s QuestTemplateSpecs) Len() int           { return len(s) }
func (s QuestTemplateSpecs) Less(i, j int) bool { return s[i].Less(s[j]) }
func (s QuestTemplateSpecs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
