// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/maruel/subcommands"
)

var cmdEditDistance = &subcommands.Command{
	UsageLine: `edit-distance [options]`,
	ShortDesc: `Computes the edit distance of two strings.`,
	LongDesc: `Computes the edit distance of two strings using insertion, deletion,
	substitution, and optionally also transposition. This expects the properties
	of the quest to be a single JSON dictionary with two keys "a" and "b". Each
	key should correspond to a string. The result of the job will be a dictionary
	with a key "dist" that is the minimum number of edit operations needed to
	transform a into b.`,
	CommandRun: func() subcommands.CommandRun {
		r := &editDistanceRun{}
		r.registerOptions()
		return r
	},
}

type editDistanceRun struct {
	cmdRun

	useTransposition bool
}

func (e *editDistanceRun) registerOptions() {
	e.cmdRun.registerOptions()
	e.Flags.BoolVar(&e.useTransposition, "use-transposition", false,
		"Include transposition as one of the possible candidates.")
}

// EditParams is the 'parameters' for the edit-distance algorithm.
type EditParams struct {
	A string `json:"a"`
	B string `json:"b"`
}

// EditResult is the normal result data for the edit-distance algorithm.
type EditResult struct {
	Distance uint32 `json:"dist"`

	// OpHistory is a string which says which edit commands were taken. The
	// symbols are:
	//   = - no edit
	//   ~ - substitute
	//   - - delete
	//   + - insert
	//   X - transpose
	//   ! - error
	OpHistory string `json:"op_history"`
	Error     string `json:"error,omitempty"`
}

func (e *editDistanceRun) Run(a subcommands.Application, args []string) int {
	e.start(a, e, cmdEditDistance)

	p := &EditParams{}
	err := json.NewDecoder(bytes.NewBufferString(e.questDesc.Parameters)).Decode(p)
	if err != nil {
		e.finish(0, "", err)
		return 0
	}

	e.finish(e.compute(p))
	return 0
}

var editSymbolLookup = map[string]string{
	"substitution":  "~",
	"insertion":     "+",
	"deletion":      "-",
	"equality":      "=",
	"transposition": "X",
}

func (e *editDistanceRun) compute(p *EditParams) (uint32, string, error) {
	// If one string is empty, the edit distance is the length of the other
	// string.
	if len(p.A) == 0 {
		return uint32(len(p.B)), strings.Repeat("+", len(p.B)), nil
	} else if len(p.B) == 0 {
		return uint32(len(p.A)), strings.Repeat("-", len(p.A)), nil
	}

	// If both strings are exactly equality, the distance between them is 0.
	if p.A == p.B {
		return 0, strings.Repeat("=", len(p.A)), nil
	}

	toDep := []interface{}{
		&EditParams{ // substitution || equality
			A: p.A[1:],
			B: p.B[1:],
		},
		&EditParams{ // deletion (remove a char from A)
			A: p.A[1:],
			B: p.B,
		},
		&EditParams{ // insertion (add the char to A)
			A: p.A,
			B: p.B[1:],
		},
	}
	if e.useTransposition && len(p.A) > 1 && len(p.B) > 1 {
		if p.A[0] == p.B[1] && p.B[0] == p.A[1] {
			toDep = append(toDep, &EditParams{ // transposition
				A: p.A[2:],
				B: p.B[2:],
			})
		}
	}

	opNames := []string{
		"substitution", "deletion", "insertion", "transposition",
	}
	if p.A[0] == p.B[0] {
		opNames[0] = "equality"
	}

	retval := uint32(math.MaxUint32)
	opname := ""
	opchain := ""
	for i, rslt := range e.depOn(toDep...) {
		result := &EditResult{}
		err := json.NewDecoder(bytes.NewBufferString(rslt)).Decode(result)
		if err != nil {
			return 0, "", err
		}

		opName := opNames[i]
		if result.Error != "" {
			return 0, "!" + result.OpHistory, fmt.Errorf(result.Error)
		}
		cost := result.Distance
		if opName != "equality" {
			// all operations (except equality) cost 1
			cost++
		}
		if cost < retval {
			retval = cost
			opname = opName
			opchain = result.OpHistory
		}
	}

	return retval, editSymbolLookup[opname] + opchain, nil
}

func (e *editDistanceRun) finish(dist uint32, opHistory string, err error) {
	if err == nil {
		e.cmdRun.finish(&EditResult{dist, opHistory, ""})
	} else {
		e.cmdRun.finish(&EditResult{Error: err.Error()})
	}
}
