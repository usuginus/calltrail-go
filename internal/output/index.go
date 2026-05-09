package output

import (
	"fmt"
	"io"
	"sort"

	"github.com/usuginus/calltrail-go/internal/model"
)

type functionIndexEntry struct {
	Layer       string
	Call        model.CallRef
	Occurrences int
}

func writeFunctionIndex(w io.Writer, flow model.APIFlow) {
	entries := buildFunctionIndexEntries(flow)
	if len(entries) == 0 {
		return
	}

	fmt.Fprintln(w, "### function index")
	fmt.Fprintln(w)
	for _, layer := range functionIndexLayerNames(entries) {
		layerEntries := functionIndexEntriesForLayer(entries, layer)
		if len(layerEntries) == 0 {
			continue
		}
		fmt.Fprintf(w, "#### %s\n", layer)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| function | location | occurrences |")
		fmt.Fprintln(w, "| --- | --- | ---: |")
		for _, entry := range layerEntries {
			fmt.Fprintf(
				w,
				"| %s | %s | %d |\n",
				tableCell(inlineCode(entry.Call.Symbol)),
				tableCell(locationCell(entry.Call)),
				entry.Occurrences,
			)
		}
		fmt.Fprintln(w)
	}
}

func buildFunctionIndexEntries(flow model.APIFlow) []functionIndexEntry {
	treeEntries := collectCallTreeEntries(flow)
	entries := make([]functionIndexEntry, 0, len(treeEntries))
	index := make(map[string]int, len(treeEntries))
	seenOccurrences := make(map[string]bool, len(treeEntries))
	layerOrder := make(map[string]int)
	for _, treeEntry := range treeEntries {
		if _, ok := layerOrder[treeEntry.Layer]; !ok {
			layerOrder[treeEntry.Layer] = len(layerOrder)
		}
		occurrenceKey := treeEntry.Layer + "\x00" + callKey(treeEntry.Call) + "\x00" + treeEntry.Call.Via
		if seenOccurrences[occurrenceKey] {
			continue
		}
		seenOccurrences[occurrenceKey] = true

		key := treeEntry.Layer + "\x00" + callKey(treeEntry.Call)
		if existingIndex, ok := index[key]; ok {
			entries[existingIndex].Occurrences++
			continue
		}
		index[key] = len(entries)
		entries = append(entries, functionIndexEntry{
			Layer:       treeEntry.Layer,
			Call:        treeEntry.Call,
			Occurrences: 1,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Layer != entries[j].Layer {
			return layerOrder[entries[i].Layer] < layerOrder[entries[j].Layer]
		}
		return callLess(entries[i].Call, entries[j].Call)
	})
	return entries
}

func functionIndexLayerNames(entries []functionIndexEntry) []string {
	var layers []string
	seen := make(map[string]bool)
	for _, entry := range entries {
		if seen[entry.Layer] {
			continue
		}
		seen[entry.Layer] = true
		layers = append(layers, entry.Layer)
	}
	return layers
}

func functionIndexEntriesForLayer(entries []functionIndexEntry, layer string) []functionIndexEntry {
	var out []functionIndexEntry
	for _, entry := range entries {
		if entry.Layer == layer {
			out = append(out, entry)
		}
	}
	return out
}

func locationCell(call model.CallRef) string {
	if call.File == "" {
		return "-"
	}
	return inlineCode(fmt.Sprintf("%s:%d", call.File, call.Line))
}
