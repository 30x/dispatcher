package router

import (
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
)

/*
WatchableResourceSet provides an interface to a k8s resource that is watchable by dispatcher.
Implementation must provide Get, Watch methods and a way to convert k8s object to a dispatch WatchableResource model
*/
type WatchableResourceSet interface {
	// Returns all current k8s resources converted to the appropriate model
	Get() ([]WatchableResource, string, error)
	// Returns a k8s watch.Interface subscribing to changes
	Watch(resouceVersion string) (watch.Interface, error)
	// Converts a k8s object into a WatchableResource per type
	ConvertToModel(interface{}) WatchableResource
	//Watchable takes in a k8s object as a generic interface and tests if the resouce should be watched by dispatcher using it's label selectors
	Watchable(interface{}) bool
	// Adds Resouce to Cache in the proper bucket based on it's type
	CacheAdd(*Cache, WatchableResource)
	// Removes a Resouce from Cache's bucket based on it's type
	CacheRemove(*Cache, string)
	// Takes a resource and compares it to the resource in the Cache based on it's id, returns true if equal. Returns false otherwise and if the cache object does not exist.
	CacheCompare(*Cache, WatchableResource) bool
	// Returns the cache id of the k8s object without having to convert it fully to the Dispatcher model
	IDFromObject(interface{}) string
}

/*
WatchableResource interface that each watchable resource most implement. Id() as the cache key and a hash method for comparison
*/
type WatchableResource interface {
	Id() string
	Hash() uint64
}

/*
Cache is the structure containing the router API Keys and the routable pods cache and namespaces
*/
type Cache struct {
	Namespaces map[string]*Namespace
	Pods       map[string]*PodWithRoutes
	Secrets    map[string]*Secret
}

/*
PodWithRoutes contains a pod and its routes
*/
type PodWithRoutes struct {
	Name      string
	Namespace string
	Status    api.PodPhase
	Routes    []*Route
	// Hash of annotation to quickly compare changes
	hash uint64
}

/*
Route describes the incoming route matching details and the outgoing proxy backend details
*/
type Route struct {
	Incoming *Incoming
	Outgoing *Outgoing
}

/*
Incoming describes the information required to route an incoming request
*/
type Incoming struct {
	Path string
}

/*
Outgoing describes the information required to proxy to a backend
*/
type Outgoing struct {
	IP         string
	Port       string
	TargetPath string
}
