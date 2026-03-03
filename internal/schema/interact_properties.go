// Purpose: Merges all interact property groups into the complete interact tool property set.
// Why: Provides the top-level assembly point for interact schema properties.
package schema

func interactToolProperties() map[string]any {
	props := make(map[string]any)
	mergeProps(props, interactDispatchProperties())
	mergeProps(props, interactTargetingProperties())
	mergeProps(props, interactCoreActionProperties())
	mergeProps(props, interactFormAndWaitProperties())
	mergeProps(props, interactOutputAndBatchProperties())
	return props
}

func mergeProps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
