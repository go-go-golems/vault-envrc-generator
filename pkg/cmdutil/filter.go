package cmdutil

// BuildSelectorSet returns a set of non-empty selector strings for quick membership checks.
func BuildSelectorSet(selectors []string) map[string]struct{} {
	set := make(map[string]struct{}, len(selectors))
	for _, s := range selectors {
		if s == "" {
			continue
		}
		set[s] = struct{}{}
	}
	return set
}

// FilterItems filters items by matching any of the provided key functions against the selector set.
// When selectors is empty or all selectors are blank, the original slice is returned.
func FilterItems[T any](items []T, selectors []string, keyFuncs ...func(T) string) []T {
	if len(items) == 0 {
		return items
	}
	set := BuildSelectorSet(selectors)
	if len(set) == 0 || len(keyFuncs) == 0 {
		return items
	}
	result := make([]T, 0, len(items))
outer:
	for _, item := range items {
		for _, keyFn := range keyFuncs {
			if keyFn == nil {
				continue
			}
			key := keyFn(item)
			if key == "" {
				continue
			}
			if _, ok := set[key]; ok {
				result = append(result, item)
				continue outer
			}
		}
	}
	return result
}
