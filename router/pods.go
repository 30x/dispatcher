package router

import (
	"encoding/json"
	"hash/fnv"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/labels"
	"k8s.io/client-go/pkg/watch"
	"log"
)

/*
PodWatchableSet implements WatchableResourceSet interface to provide access to k8s pod resouces.
*/
type PodWatchableSet struct {
	Config     *Config
	KubeClient *kubernetes.Clientset
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
HealthCheck of an Outgoing upstream server allows nginx to monitor pod health.
*/
type HealthCheck struct {
	HTTPCheck          bool
	Path               string
	Method             string
	TimeoutMs          int32
	IntervalMs         int32
	UnhealthyThreshold int32
	HealthyThreshold   int32
	Port               int32
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
	IP          string
	Port        string
	TargetPath  string
	HealthCheck *HealthCheck
}

type pathAnnotation struct {
	BasePath      string  `json:"basePath"`
	ContainerPort string  `json:"containerPort"`
	TargetPath    *string `json:"targetPath,omitempty"`
}

/*
ID returns the namespace's name
*/
func (pod PodWithRoutes) ID() string {
	return pod.Name
}

/*
Hash returns the stored version of all the annotations hashed using fnv
*/
func (pod PodWithRoutes) Hash() uint64 {
	return pod.hash
}

/*
GetPodRoutes returns an array of routes defined within the provided pod
*/
func GetRoutes(config *Config, pod *api.Pod) []*Route {
	var routes []*Route
	// Do not process pods that are not running
	if pod.Status.Phase != api.PodRunning {
		return routes
	}

	// Do not process pods without an IP
	if pod.Status.PodIP == "" {
		return routes
	}

	annotation, ok := pod.Annotations[config.PodsPathsAnnotation]
	// This pod does not have the hosts annotation set
	if !ok {
		return routes
	}

	var tmpPaths []pathAnnotation
	err := json.Unmarshal([]byte(annotation), &tmpPaths)
	if err != nil {
		log.Printf("Pod %s in Namespace %s had issue parsing json path annotation %s.\n", pod.Name, pod.Namespace, config.PodsPathsAnnotation)
		return routes
	}

	return routes
}

/*
Watch returns a k8s watch.Interface that subscribes to any pod changes
*/
func (s PodWatchableSet) Watch(resouceVersion string) (watch.Interface, error) {
	// Get the list options so we can create the watch
	watchOptions := api.ListOptions{
		LabelSelector:   s.Config.PodsRoutableLabelSelector,
		ResourceVersion: resouceVersion,
	}

	// Create a watcher to be notified of Namespace events
	watcher, err := s.KubeClient.Core().Pods(api.NamespaceAll).Watch(watchOptions)
	if err != nil {
		return nil, err
	}

	return watcher, nil
}

/*
Get returns a list of Namespace in the form of a WatchableResource interface and a k8s resource version. If any k8s client errors occur it is returned.
*/
func (s PodWatchableSet) Get() ([]WatchableResource, string, error) {
	// Query the initial list of Namespaces
	k8sPods, err := s.KubeClient.Core().Pods(api.NamespaceAll).List(api.ListOptions{
		LabelSelector: s.Config.PodsRoutableLabelSelector,
	})
	if err != nil {
		return nil, "", err
	}

	pods := make([]WatchableResource, len(k8sPods.Items))

	for i, pod := range k8sPods.Items {
		pods[i] = s.ConvertToModel(&pod)
	}

	return pods, k8sPods.ListMeta.ResourceVersion, nil
}

/*
ConvertToModel takes in a k8s *api.Pod as a blank interface and converts it to a Namespace as a WatchableResource
*/
func (s PodWatchableSet) ConvertToModel(in interface{}) WatchableResource {
	pod := in.(*api.Pod)
	return &PodWithRoutes{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    pod.Status.Phase,
		Routes:    GetRoutes(s.Config, pod),
		hash:      calculatePodHash(s.Config, pod),
	}
}

/*
Watchable tests where the *api.Pod has the routable label selector for the pod to be watched.
*/
func (s PodWatchableSet) Watchable(in interface{}) bool {
	// TODO: add label.Selector on config to avoid parsing on every comparison
	// Ignore err we've already checked in the config
	selector, _ := labels.Parse(s.Config.PodsRoutableLabelSelector)
	pod := in.(*api.Pod)
	matched := selector.Matches(labels.Set(pod.Labels))
	if !matched {
		return false
	}

	if pod.Status.Phase != api.PodRunning {
		return false
	}

	return true
}

/*
CacheAdd adds Namespace to the cache's namespace bucket
*/
func (s PodWatchableSet) CacheAdd(cache *Cache, item WatchableResource) {
	pod := item.(*PodWithRoutes)
	cache.Pods[item.ID()] = pod
}

/*
CacheRemove removes the Namespace using the id given from the Cache's Namespaces bucket
*/
func (s PodWatchableSet) CacheRemove(cache *Cache, id string) {
	delete(cache.Pods, id)
}

/*
CacheCompare compares the given Pod with the pod in the cache, if equal returns true otherwise returns false. If cache value does not exist return false.
*/
func (s PodWatchableSet) CacheCompare(cache *Cache, newItem WatchableResource) bool {
	item, ok := cache.Pods[newItem.ID()]
	if !ok {
		return false
	}

	//TODO: Compare Pod status

	return item.Hash() == newItem.Hash()
}

/*
IDFromObject returns the Pods' name from the *api.Pod object
*/
func (s PodWatchableSet) IDFromObject(in interface{}) string {
	pod := in.(*api.Pod)
	return pod.Name
}

/*
 calculatePodHash calculates hash for all annotations of the pod
*/
func calculatePodHash(config *Config, pod *api.Pod) uint64 {
	h := fnv.New64()
	h.Write([]byte(pod.Annotations[config.PodsPathsAnnotation]))
	h.Write([]byte(pod.Namespace))
	h.Write([]byte(pod.Status.Phase))
	h.Write([]byte(pod.Name))
	h.Write([]byte(pod.Status.PodIP))
	// TODO: Add healthcheck to hash
	return h.Sum64()
}
