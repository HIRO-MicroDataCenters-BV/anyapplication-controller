package moutils

import "github.com/samber/mo"

// Map returns a new `mo.Option` wrapping the result of applying `f` to the value of opt, if present, and None otherwise.
func Map[I any, O any](opt mo.Option[I], f func(I) O) mo.Option[O] {
	if val, ok := opt.Get(); ok {
		return mo.Some(f(val))
	}

	return mo.None[O]()
}
