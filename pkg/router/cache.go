package router

import (
	"fmt"
)

/*
AddResourceToCache adds a WatchableResource type to the cache based on it's underlying type. If type is not recognized it returns an error.
*/
func AddResourceToCache(cache *Cache, item WatchableResource) error {
	switch item.(type) {
	case Namespace:
		// Assert to namespace variable and save in cache
		ns := item.(Namespace)
		cache.Namespaces[item.Id()] = &ns
	case Secret:
		// Assert to namespace variable and save in cache
		secret := item.(Secret)
		cache.Secrets[item.Id()] = &secret
	default:
		return fmt.Errorf("Unknown item type for cache.")
	}

	return nil
}
