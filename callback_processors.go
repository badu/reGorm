package gorm

import "fmt"

func (cps *CallbacksProcessors) add(proc ...*CallbacksProcessor) {
	*cps = append(*cps, proc...)
}

func (cps *CallbacksProcessors) len() int {
	return len(*cps)
}

func (cps *CallbacksProcessors) reorder(ofCallback *Callbacks) {
	var creates, updates, deletes, queries, rowQueries CallbacksProcessors
	//collect processors by their kind
	for _, processor := range *cps {
		if processor.name != "" {
			switch processor.kind {
			case cb_create:
				creates.add(processor)
			case cb_update:
				updates.add(processor)
			case cb_delete:
				deletes.add(processor)
			case cb_query:
				queries.add(processor)
			case cb_row:
				rowQueries.add(processor)
			}
		}
	}
	//avoid unnecessary calls
	if creates.len() > 0 {
		ofCallback.creates = creates.sortProcessors()
	}
	if updates.len() > 0 {
		ofCallback.updates = updates.sortProcessors()
	}
	if deletes.len() > 0 {
		ofCallback.deletes = deletes.sortProcessors()
	}
	if queries.len() > 0 {
		ofCallback.queries = queries.sortProcessors()
	}
	if rowQueries.len() > 0 {
		ofCallback.rowQueries = rowQueries.sortProcessors()
	}

}

// sortProcessors sort callback processors based on its before, after, remove, replace
func (cps CallbacksProcessors) sortProcessors() ScopedFuncs {
	var allNames, sortedNames StrSlice
	var sortedFuncs ScopedFuncs

	for _, cp := range cps {
		// show warning message the callback name already exists
		if index := allNames.rIndex(cp.name); index > -1 && !cp.replace && !cp.remove {
			fmt.Printf("[warning] duplicated callback `%v` from %v\n", cp.name, fileWithLineNum())
		}
		allNames.add(cp.name)
	}
	for _, cp := range cps {
		cp.sortCallbackProcessor(&allNames, &sortedNames, cps)
	}

	for _, name := range sortedNames {
		if index := allNames.rIndex(name); !cps[index].remove {
			sortedFuncs.add(cps[index].processor)
		}
	}
	return sortedFuncs
}
