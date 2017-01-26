package router

import (
	"k8s.io/client-go/pkg/watch"
)

/*
ProcessEvent takes in a WatchableResourceSet and a k8s watch.Event and runs the logic to either add/remove/update the cache with the new resouce if needed. Returns true if cache has changed and nginx needs a restart.
 - If the Resource is delete from k8s the cache entry is removed and a restart is needed.
 - If the Resource is added to k8s it's added to the cache and a restart is needed.
 - If the Resouce is updated in k8s it's checked if the resouce is still watchable, if not it's removed from the cache. If it's still watchable and not equal to the cached object it's replaced and a restart is needed.
*/
func ProcessEvent(cache *Cache, resourceType WatchableResourceSet, event watch.Event) bool {
	switch event.Type {
	case watch.Added:
		if resourceType.Watchable(event.Object) {
			// Resource Added add to cache
			newResource := resourceType.ConvertToModel(event.Object)
			resourceType.CacheAdd(cache, newResource)
			return true
		}
	case watch.Deleted:
		// Resource delete try and remove from cache
		resourceType.CacheRemove(cache, resourceType.IDFromObject(event.Object))
		// TODO: What if the resource was never in the cache, should we not restart?
		return true
	case watch.Modified:
		// Test if the resouce is still watchable
		if resourceType.Watchable(event.Object) {
			// Compare resource to resource in cache
			newResource := resourceType.ConvertToModel(event.Object)
			if !resourceType.CacheCompare(cache, newResource) {
				// Resource has been modified, update cache and restart
				resourceType.CacheAdd(cache, newResource)
				return true
			}
		} else {
			// If Resource is no longer watchable remove from cache
			resourceType.CacheRemove(cache, resourceType.IDFromObject(event.Object))
			return true
		}
	}

	// Nothing's changed don't restart
	return false
}

/*
NewCache returns a create a Cache object and returns a pointer to the object
*/
func NewCache() *Cache {
	return &Cache{
		Namespaces: make(map[string]*Namespace),
		Pods:       make(map[string]*PodWithRoutes),
		Secrets:    make(map[string]*Secret),
	}
}
