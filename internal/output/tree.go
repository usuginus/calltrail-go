package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
)

type callTreeEntry struct {
	Layer string
	Call  model.CallRef
}

type callTreeNode struct {
	Layer    string
	Call     model.CallRef
	Children []callTreeNode
}

func writeCallTree(w io.Writer, flow model.APIFlow) {
	root := buildCallTree(flow)
	fmt.Fprintln(w, "### call tree")
	fmt.Fprintln(w)
	writeCallTreeNode(w, root, 0)
	fmt.Fprintln(w)
}

func buildCallTree(flow model.APIFlow) callTreeNode {
	root := callTreeNode{
		Layer: "handler",
		Call: model.CallRef{
			Symbol: flow.Entrypoint.Symbol,
			File:   flow.Entrypoint.File,
			Line:   flow.Entrypoint.Line,
		},
	}

	entries := collectCallTreeEntries(flow)
	if len(entries) == 0 {
		return root
	}

	knownSymbols := map[string]bool{root.Call.Symbol: true}
	for _, entry := range entries {
		knownSymbols[entry.Call.Symbol] = true
	}

	childrenByVia := make(map[string][]callTreeEntry)
	symbolFiles := callTreeSymbolFiles(entries)
	var rootChildren []callTreeEntry
	for _, entry := range entries {
		switch {
		case entry.Call.Via == "", entry.Call.Via == root.Call.Symbol:
			rootChildren = append(rootChildren, entry)
		case knownSymbols[entry.Call.Via]:
			childrenByVia[entry.Call.Via] = append(childrenByVia[entry.Call.Via], entry)
		}
	}

	root.Children = buildCallTreeChildren(rootChildren, childrenByVia, symbolFiles, map[string]bool{root.Call.Symbol: true})
	return root
}

func buildCallTreeChildren(
	entries []callTreeEntry,
	childrenByVia map[string][]callTreeEntry,
	symbolFiles map[string]map[string]bool,
	path map[string]bool,
) []callTreeNode {
	entries = sortCallTreeEntries(dedupeCallTreeEntries(entries))
	nodes := make([]callTreeNode, 0, len(entries))
	for _, entry := range entries {
		node := callTreeNode{Layer: entry.Layer, Call: entry.Call}
		if !path[entry.Call.Symbol] {
			nextPath := copySymbolPath(path)
			nextPath[entry.Call.Symbol] = true
			children := callTreeChildrenForParent(entry, childrenByVia[entry.Call.Symbol], symbolFiles)
			node.Children = buildCallTreeChildren(children, childrenByVia, symbolFiles, nextPath)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func callTreeChildrenForParent(parent callTreeEntry, children []callTreeEntry, symbolFiles map[string]map[string]bool) []callTreeEntry {
	if len(children) <= 1 || len(symbolFiles[parent.Call.Symbol]) <= 1 {
		return children
	}

	sameFile := make([]callTreeEntry, 0, len(children))
	for _, child := range children {
		if child.Call.File == parent.Call.File {
			sameFile = append(sameFile, child)
		}
	}
	if len(sameFile) > 0 {
		return sameFile
	}
	return nil
}

func writeCallTreeNode(w io.Writer, node callTreeNode, depth int) {
	fmt.Fprintf(w, "%s- [%s] %s\n", strings.Repeat("  ", depth), node.Layer, callReference(node.Call))
	for _, child := range node.Children {
		writeCallTreeNode(w, child, depth+1)
	}
}

func collectCallTreeEntries(flow model.APIFlow) []callTreeEntry {
	var entries []callTreeEntry
	for _, layer := range flow.Trail.Layers {
		entries = appendLayerCallTreeEntries(entries, layer.Name, layer.Calls)
	}
	for _, branch := range flow.Trail.Branches {
		for _, branchCase := range branch.Cases {
			for _, layer := range branchCase.Layers {
				entries = appendLayerCallTreeEntries(entries, layer.Name, layer.Calls)
			}
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		for _, dispatchCase := range dispatch.Cases {
			for _, layer := range dispatchCase.Layers {
				entries = appendLayerCallTreeEntries(entries, layer.Name, layer.Calls)
			}
		}
	}
	entries = appendLayerCallTreeEntries(entries, "async", flow.Trail.Async)
	entries = appendLayerCallTreeEntries(entries, "other", summarizeUnknown(collectUnknownCalls(flow), operationCallsiteSymbols(flow)))
	return filterCallTreeEntries(entries)
}

func appendLayerCallTreeEntries(entries []callTreeEntry, layer string, calls []model.CallRef) []callTreeEntry {
	if layer == "" {
		layer = "other"
	}
	for _, call := range calls {
		if call.Symbol == "" {
			continue
		}
		entries = append(entries, callTreeEntry{Layer: layer, Call: call})
	}
	return entries
}

func filterCallTreeEntries(entries []callTreeEntry) []callTreeEntry {
	hasChildren := make(map[string]bool)
	for _, entry := range entries {
		if entry.Call.Via != "" {
			hasChildren[entry.Call.Via] = true
		}
	}

	var out []callTreeEntry
	for _, entry := range entries {
		if isInternalHelperCall(entry.Call) && !hasChildren[entry.Call.Symbol] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func callTreeSymbolFiles(entries []callTreeEntry) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.Call.Symbol == "" || entry.Call.File == "" {
			continue
		}
		if out[entry.Call.Symbol] == nil {
			out[entry.Call.Symbol] = make(map[string]bool)
		}
		out[entry.Call.Symbol][entry.Call.File] = true
	}
	return out
}

func collectUnknownCalls(flow model.APIFlow) []model.CallRef {
	calls := append([]model.CallRef(nil), flow.Trail.Unknown...)
	for _, branch := range flow.Trail.Branches {
		for _, branchCase := range branch.Cases {
			calls = append(calls, branchCase.Unknown...)
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		for _, dispatchCase := range dispatch.Cases {
			calls = append(calls, dispatchCase.Unknown...)
		}
	}
	return calls
}

func sortCallTreeEntries(entries []callTreeEntry) []callTreeEntry {
	out := append([]callTreeEntry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if !sameCall(out[i].Call, out[j].Call) || out[i].Call.Depth != out[j].Call.Depth {
			return callTreeEntryLess(out[i], out[j])
		}
		return out[i].Layer < out[j].Layer
	})
	return out
}

func callTreeEntryLess(left callTreeEntry, right callTreeEntry) bool {
	if left.Call.Depth != right.Call.Depth {
		return left.Call.Depth < right.Call.Depth
	}
	if !sameCall(left.Call, right.Call) {
		return callLess(left.Call, right.Call)
	}
	return left.Layer < right.Layer
}

func dedupeCallTreeEntries(entries []callTreeEntry) []callTreeEntry {
	var out []callTreeEntry
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		key := entry.Layer + "\x00" + callKey(entry.Call)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}

func copySymbolPath(path map[string]bool) map[string]bool {
	out := make(map[string]bool, len(path)+1)
	for symbol := range path {
		out[symbol] = true
	}
	return out
}
