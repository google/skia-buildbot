package config_vars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestVarsValidate(t *testing.T) {

	test := func(fn func(*Vars), expectErr string) {
		v := FakeVars()
		fn(v)
		err := v.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr), err)
		}
	}

	// OK.
	test(func(v *Vars) {}, "")

	// Missing Branches.
	test(func(v *Vars) {
		v.Branches = nil
	}, "Branches is required")

	// Should call Branches.Validate().
	test(func(v *Vars) {
		v.Branches.Chromium = nil
	}, "Chromium is required.")
}

func TestBranchesValidate(t *testing.T) {

	test := func(fn func(*Branches), expectErr string) {
		b := FakeVars().Branches
		fn(b)
		err := b.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr), err)
		}
	}

	// OK.
	test(func(b *Branches) {}, "")

	// Missing Chromium.
	test(func(b *Branches) {
		b.Chromium = nil
	}, "Chromium is required")

	// Should call chrome_branch.Branches.Validate().
	test(func(b *Branches) {
		b.Chromium.Beta = nil
	}, "Beta branch is missing")

	// Missing CrOS4WeekBeta.
	test(func(b *Branches) {
		b.CrOS4WeekBeta = nil
	}, "CrOS4WeekBeta branch is missing")

	// Odd CrOS4WeekBeta.
	test(func(b *Branches) {
		b.CrOS4WeekBeta.Milestone = 81
	}, "CrOS4WeekBeta milestone must be even")

	// Missing CrOS4WeekStable.
	test(func(b *Branches) {
		b.CrOS4WeekStable = nil
	}, "CrOS4WeekStable branch is missing")

	// Odd CrOS4WeekStable.
	test(func(b *Branches) {
		b.CrOS4WeekStable.Milestone = 79
	}, "CrOS4WeekStable milestone must be even")
}

func TestComputeCrOS4WeekBranches(t *testing.T) {
	active := []*chrome_branch.Branch{
		{Milestone: 155, Number: 8060, Ref: "refs/branch-heads/8060", V8Branch: "15.5"},
		{Milestone: 154, Number: 8037, Ref: "refs/branch-heads/8037", V8Branch: "15.4"},
		{Milestone: 153, Number: 8010, Ref: "refs/branch-heads/8010", V8Branch: "15.3"},
		{Milestone: 152, Number: 7977, Ref: "refs/branch-heads/7977", V8Branch: "15.2"},
		{Milestone: 151, Number: 7922, Ref: "refs/branch-heads/7922", V8Branch: "15.1"},
	}
	beta, stable := computeCrOS4WeekBranches(active)
	require.NotNil(t, beta)
	require.Equal(t, 154, beta.Milestone)
	require.Equal(t, 8037, beta.Number)
	require.NotNil(t, stable)
	require.Equal(t, 152, stable.Milestone)
	require.Equal(t, 7977, stable.Number)
}

func TestTemplate(t *testing.T) {

	raw := "refs/branch-heads/{{.Branches.Chromium.Beta.Number}}"
	tmpl, err := NewTemplate(raw)
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	require.NoError(t, tmpl.Validate())

	// JSON encode/decode.
	cfgJson := fmt.Sprintf(`{"tmpl":"%s"}`, raw)
	var cfg1, cfg2 struct {
		T *Template `json:"tmpl"`
	}
	require.NoError(t, json.NewDecoder(bytes.NewReader([]byte(cfgJson))).Decode(&cfg1))
	require.NoError(t, cfg1.T.Validate())
	b, err := json.Marshal(cfg1)
	require.NoError(t, err)
	require.Equal(t, cfgJson, string(b))
	require.NoError(t, json.NewDecoder(bytes.NewReader(b)).Decode(&cfg2))
	assertdeep.Equal(t, cfg1, cfg2)

	// Update() and String().
	require.Equal(t, "", tmpl.String())
	v := FakeVars()
	require.NoError(t, tmpl.Update(v))
	require.Equal(t, "refs/branch-heads/4044", tmpl.String())
	v.Branches.Chromium.Beta.Number = 5000
	require.Equal(t, "refs/branch-heads/4044", tmpl.String())
	require.NoError(t, tmpl.Update(v))
	require.Equal(t, "refs/branch-heads/5000", tmpl.String())

	// Invalid template string.
	invalid := func(raw, expect string) {
		_, err := NewTemplate(raw)
		require.NotNil(t, err)
		require.True(t, strings.Contains(err.Error(), expect), err)
	}
	invalid("", "Template is missing")
	invalid("{{.Bogus}}", "can't evaluate field Bogus")

	// Banned template string.
	invalid("refs/branch-heads/{{.Branches.Chromium.Master.Number}}", "Templates should not use \"Chromium.Master.Number\"")
}

func TestRegistry(t *testing.T) {

	ctx := context.Background()
	v := FakeVars()
	cbc := &mocks.Client{}
	cbc.On("Get", ctx).Return(v.Branches.Chromium, v.Branches.ActiveMilestones, nil)
	r, err := NewRegistry(ctx, cbc)
	require.NoError(t, err)

	// Register().
	t1, err := NewTemplate("refs/branch-heads/{{.Branches.Chromium.Beta.Number}}")
	require.NoError(t, err)
	require.NoError(t, r.Register(t1))
	require.Equal(t, t1.String(), "refs/branch-heads/4044")

	t2, err := NewTemplate("refs/branch-heads/{{.Branches.Chromium.Stable.Number}}")
	require.NoError(t, err)
	require.NoError(t, r.Register(t2))
	require.Equal(t, t2.String(), "refs/branch-heads/3987")

	// Update().
	v.Branches.Chromium.Beta.Number = 5000
	v.Branches.Chromium.Stable.Number = 4044
	require.NoError(t, r.Update(ctx))
	require.Equal(t, t1.String(), "refs/branch-heads/5000")
	require.Equal(t, t2.String(), "refs/branch-heads/4044")
}
