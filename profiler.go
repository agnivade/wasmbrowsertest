package main

import (
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/chromedp/cdproto/profiler"
	"github.com/google/pprof/profile"
)

// locMeta is a wrapper around profile.Location with an extra
// pointer towards its parent node.
type locMeta struct {
	loc    *profile.Location
	parent *profile.Location
}

// WriteProfile converts a chromedp profile to a pprof profile.
func WriteProfile(cProf *profiler.Profile, w io.Writer, funcMap map[int]string) error {
	// Creating an empty pprof object
	pProf := profile.Profile{
		SampleType: []*profile.ValueType{
			{
				Type: "samples",
				Unit: "count",
			},
			{
				Type: "cpu",
				Unit: "nanoseconds",
			},
		},
		PeriodType: &profile.ValueType{
			Type: "cpu",
			Unit: "nanoseconds",
		},
		TimeNanos:     int64(cProf.StartTime) * 1000,
		DurationNanos: int64(cProf.EndTime-cProf.StartTime) * 1000,
	}

	// Helper maps which allow easy construction of the profile.
	fnMap := make(map[string]*profile.Function)
	locMap := make(map[int64]locMeta)
	funcRegexp := regexp.MustCompile(`^wasm-function\[([0-9]+)\]$`)

	// A monotonically increasing function ID.
	// We bump this everytime we see a new function.
	var fnID uint64 = 1
	pProf.Location = make([]*profile.Location, len(cProf.Nodes))
	// Now we iterate the cprof nodes and populate the functions and locations.
	for i, n := range cProf.Nodes {
		cf := n.CallFrame
		// We create such a function key to uniquely map functions, since the profile does not have
		// any unique function ID.
		fnKey := cf.FunctionName + strconv.Itoa(int(cf.LineNumber)) + strconv.Itoa(int(cf.ColumnNumber))
		pFn, exists := fnMap[fnKey]
		if !exists {
			// If the function name is of form wasm-function[], then we find out the actual function name
			// from the passed map and replace the name.
			if funcRegexp.MatchString(cf.FunctionName) {
				fIndex, err := strconv.Atoi(funcRegexp.FindStringSubmatch(cf.FunctionName)[1])
				if err != nil {
					return fmt.Errorf("incorrect wasm function name: %s", cf.FunctionName)
				}
				cf.FunctionName = funcMap[fIndex]
			}

			// Creating the function struct
			pFn = &profile.Function{
				ID:         fnID,
				Name:       cf.FunctionName,
				SystemName: cf.FunctionName,
				Filename:   cf.URL,
			}
			fnID++
			// Add it to map
			fnMap[fnKey] = pFn

			// Adding it to the function slice
			pProf.Function = append(pProf.Function, pFn)
		}

		loc := &profile.Location{
			ID: uint64(n.ID),
			Line: []profile.Line{
				{
					Function: pFn,
					Line:     cf.LineNumber,
				},
			},
		}

		// Add it to map
		locMap[n.ID] = locMeta{loc: loc}

		// Add it to location slice
		pProf.Location[i] = loc
	}

	// Iterate it once more, to build the parent-child chain.
	for _, n := range cProf.Nodes {
		parent := locMap[n.ID]
		for _, childID := range n.Children {
			child := locMap[childID]
			child.parent = parent.loc
			locMap[childID] = child
		}
	}

	// Final iteration to construct the sample list
	pProf.Sample = make([]*profile.Sample, len(cProf.Samples))
	for i, id := range cProf.Samples {
		node := locMap[id]
		sample := profile.Sample{}
		sample.Value = []int64{1, 100000} // XXX: How to get the integer values from ValueType ??
		// walk up the parent chain, and add locations.
		leaf := node.loc
		parent := node.parent
		sample.Location = append(sample.Location, leaf)
		for parent != nil {
			sample.Location = append(sample.Location, parent)
			parent = locMap[int64(parent.ID)].parent
		}
		// Add it to sample slice
		pProf.Sample[i] = &sample
	}

	err := pProf.Write(w)
	if err != nil {
		return err
	}
	return pProf.CheckValid()
}
