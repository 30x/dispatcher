package router

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/labels"
	"k8s.io/client-go/pkg/watch"
	"log"
	"regexp"
)

const (
	hostnameRegexStr = "^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$"
	ipRegexStr       = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
)

var hostnameRegex *regexp.Regexp
var ipRegex *regexp.Regexp

/*
init builds all regex needed for validation
*/
func init() {
	// Compile all regular expressions
	hostnameRegex = compileRegex(hostnameRegexStr)
	ipRegex = compileRegex(ipRegexStr)
}

/*
NamespaceWatchableSet implements WatchableResourceSet interface to provide access to k8s namespace resouces.
*/
type NamespaceWatchableSet struct {
	Config     *Config
	KubeClient *kubernetes.Clientset
}

/*
Namespace describes the information stored on the k8s namespace object for routing
*/
type Namespace struct {
	Name         string
	Hosts        map[string]HostOptions
	Organization string
	Environment  string
	// Hash of annotation to quickly compare changes
	hash uint64
}

// SecretRef ref to secret field
type SecretRef struct {
	Key string `json:"key"`
}

// ValueFrom specify reference to a secret
type ValueFrom struct {
	SecretKeyRef *SecretRef `json:"secretKeyRef,omitempty"`
}

// OptionValue value from ref
type OptionValue struct {
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

/*
SSLOptions contains options for the host
*/
type SSLOptions struct {
	Certificate      OptionValue  `json:"certificate"`
	Key              OptionValue  `json:"certificateKey"`
	ClientCertficate *OptionValue `json:"clientCertificate,omitempty"`
}

/*
HostOptions contains any options for the host. Nothing right now.
*/
type HostOptions struct {
	SSLOptions *SSLOptions `json:"ssl,omitempty"`
}

/*
ID returns the namespace's name
*/
func (ns Namespace) ID() string {
	return ns.Name
}

/*
Hash returns the stored version of all the annotations hashed using fnv
*/
func (ns Namespace) Hash() uint64 {
	return ns.hash
}

/*
Watch returns a k8s watch.Interface that subscribes to any namespace changes
*/
func (s NamespaceWatchableSet) Watch(resouceVersion string) (watch.Interface, error) {
	// Get the list options so we can create the watch
	namespacesWatchOptions := api.ListOptions{
		LabelSelector:   s.Config.RoutableLabelSelector,
		ResourceVersion: resouceVersion,
	}

	// Create a watcher to be notified of Namespace events
	watcher, err := s.KubeClient.Core().Namespaces().Watch(namespacesWatchOptions)
	if err != nil {
		return nil, err
	}

	return watcher, nil
}

/*
Get returns a list of Namespace in the form of a WatchableResource interface and a k8s resource version. If any k8s client errors occur it is returned.
*/
func (s NamespaceWatchableSet) Get() ([]WatchableResource, string, error) {
	// Query the initial list of Namespaces
	k8sNamespaces, err := s.KubeClient.Core().Namespaces().List(api.ListOptions{
		LabelSelector: s.Config.RoutableLabelSelector,
	})
	if err != nil {
		return nil, "", err
	}

	namespaces := make([]WatchableResource, len(k8sNamespaces.Items))

	for i, ns := range k8sNamespaces.Items {
		namespaces[i] = s.ConvertToModel(&ns)
	}

	return namespaces, k8sNamespaces.ListMeta.ResourceVersion, nil
}

/*
ConvertToModel takes in a k8s *api.Namespace as a blank interface and converts it to a Namespace as a WatchableResource
*/
func (s NamespaceWatchableSet) ConvertToModel(in interface{}) WatchableResource {
	namespace := in.(*api.Namespace)
	ns := &Namespace{
		Name:         namespace.Name,
		Hosts:        getHostsFromNamespace(s.Config, namespace),
		Organization: namespace.Labels[s.Config.NamespaceOrgLabel],
		Environment:  namespace.Labels[s.Config.NamespaceEnvLabel],
		hash:         calculateNamespaceHash(s.Config, namespace),
	}
	return ns
}

/*
Watchable tests where the *api.Namespace has the routable label selector for the namespace to be watched.
*/
func (s NamespaceWatchableSet) Watchable(in interface{}) bool {
	// TODO: add label.Selector on config to avoid parsing on every comparison
	// Ignore err we've already checked in the config
	selector, _ := labels.Parse(s.Config.RoutableLabelSelector)
	namespace := in.(*api.Namespace)
	return selector.Matches(labels.Set(namespace.Labels))
}

/*
CacheAdd adds Namespace to the cache's namespace bucket
*/
func (s NamespaceWatchableSet) CacheAdd(cache *Cache, item WatchableResource) {
	namespace := item.(*Namespace)
	cache.Namespaces[item.ID()] = namespace
}

/*
CacheRemove removes the Namespace using the id given from the Cache's Namespaces bucket
*/
func (s NamespaceWatchableSet) CacheRemove(cache *Cache, id string) {
	delete(cache.Namespaces, id)
}

/*
CacheCompare compares the given Namespace with the namespace in the cache, if equal returns true otherwise returns false. If cache value does not exist return false.
*/
func (s NamespaceWatchableSet) CacheCompare(cache *Cache, newItem WatchableResource) bool {
	item, ok := cache.Namespaces[newItem.ID()]
	if !ok {
		return false
	}
	return item.Hash() == newItem.Hash()
}

/*
IDFromObject returns the Namespaces' name from the *api.Namespace object
*/
func (s NamespaceWatchableSet) IDFromObject(in interface{}) string {
	namespace := in.(*api.Namespace)
	return namespace.Name
}

/*
GetHostsFromNamespace returns all valid hosts from configured host annotation on Namespace
*/
func getHostsFromNamespace(config *Config, namespace *api.Namespace) map[string]HostOptions {
	var hosts = map[string]HostOptions{}

	annotation, ok := namespace.Annotations[config.NamespaceHostsAnnotation]
	if !ok {
		return hosts
	}

	err := json.Unmarshal([]byte(annotation), &hosts)
	if err != nil {
		log.Printf("Namespace (%s) host issue: %s is not a valid JSON %v\n", namespace.Name, config.NamespaceHostsAnnotation, err)
		return hosts
	}

	// Process the routing hosts
	for host, options := range hosts {
		valid := hostnameRegex.MatchString(host)

		if !valid {
			valid = ipRegex.MatchString(host)

			if !valid {
				delete(hosts, host)
				log.Printf("Namespace (%s) host issue: %s (%s) is not a valid hostname/ip\n", namespace.Name, config.NamespaceHostsAnnotation, host)
				continue
			}

			// If host has ssl options validate options
			if options.SSLOptions != nil {
				if err := validSSLOptions(options.SSLOptions); err != nil {
					delete(hosts, host)
					log.Printf("Namespace (%s) ssl options issue: %v\n", namespace.Name, err)
				}
			}

		}
	}
	return hosts
}

func validSSLOptions(opts *SSLOptions) error {
	if opts.Certificate.ValueFrom == nil {
		return fmt.Errorf("certificate option missing valueFrom")
	}
	if opts.Certificate.ValueFrom.SecretKeyRef == nil {
		return fmt.Errorf("certificate option missing secretKeyRef")
	}

	if opts.Key.ValueFrom == nil {
		return fmt.Errorf("key option missing valueFrom")
	}
	if opts.Key.ValueFrom.SecretKeyRef == nil {
		return fmt.Errorf("key option missing secretKeyRef")
	}

	if opts.ClientCertficate != nil {
		if opts.ClientCertficate.ValueFrom == nil {
			return fmt.Errorf("client certificate option missing valueFrom")
		}
		if opts.ClientCertficate.ValueFrom.SecretKeyRef == nil {
			return fmt.Errorf("client certificate option missing secretKeyRef")
		}
	}

	return nil
}

/*
compileRegex returns a regex object from a regex string
*/
func compileRegex(regexStr string) *regexp.Regexp {
	compiled, err := regexp.Compile(regexStr)

	if err != nil {
		log.Fatalf("Failed to compile regular expression (%s): %v\n", regexStr, err)
	}

	return compiled
}

/*
 calculateNamespaceHash calculates hash for hosts and paths annotations to compare when Namespace is modified.
*/
func calculateNamespaceHash(config *Config, ns *api.Namespace) uint64 {
	h := fnv.New64()
	h.Write([]byte(ns.Annotations[config.NamespaceHostsAnnotation]))
	h.Write([]byte(ns.Labels[config.NamespaceOrgLabel]))
	h.Write([]byte(ns.Labels[config.NamespaceEnvLabel]))
	return h.Sum64()
}
