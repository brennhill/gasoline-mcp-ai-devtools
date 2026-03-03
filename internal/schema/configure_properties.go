// Purpose: Merges core and runtime properties into the complete configure tool property set.
// Why: Provides the top-level assembly point for configure schema properties.
package schema

func configureToolProperties() map[string]any {
	props := make(map[string]any)
	mergeConfigureProps(props, configureCoreProperties())
	mergeConfigureProps(props, configureRuntimeProperties())
	return props
}

func mergeConfigureProps(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
