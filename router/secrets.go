package router

import (
	"hash/fnv"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
)

/*
SecretWatchableSet struct to implement the WatchableResourceSet interface. Has pointers to the config and kubernetes api.
*/
type SecretWatchableSet struct {
	Config     *Config
	KubeClient *kubernetes.Clientset
}

/*
Secret struct implements a WatchableResource interface and contains the Namespace name and Data inside the secret
*/
type Secret struct {
	Namespace  string
	RoutingKey *[]byte
	Fields     map[string][]byte // raw data values for each field in secret
	hash       uint64
}

/*
ID returns the namespace name
*/
func (s Secret) ID() string {
	return s.Namespace
}

/*
Hash returns a fnv hasve of the Secret Data
*/
func (s Secret) Hash() uint64 {
	return s.hash
}

func calculateSecretHash(s *Secret) uint64 {
	h := fnv.New64()
	if s.RoutingKey != nil {
		h.Write(*s.RoutingKey)
	}
	for _, v := range s.Fields {
		h.Write(v)
	}
	return h.Sum64()
}

/*
Watch returns a k8s watch.Interface that subscribes secretes that change in any namespace
*/
func (s SecretWatchableSet) Watch(resouceVersion string) (watch.Interface, error) {
	// Get the list options so we can create the watch
	watchOptions := api.ListOptions{
		ResourceVersion: resouceVersion,
	}

	// Create a watcher to be notified of Namespace events
	// TODO: Limit namespaces to only namespaces with label
	watcher, err := s.KubeClient.Core().Secrets(api.NamespaceAll).Watch(watchOptions)
	if err != nil {
		return nil, err
	}

	return watcher, nil
}

/*
Get returns a list of Secrets in form of WatchableResources interfaces and a k8s resource version. If any error occurs it is returned from k8s client.
*/
func (s SecretWatchableSet) Get() ([]WatchableResource, string, error) {
	// Query the initial list of Namespaces
	// TODO: Limit namespaces to only namespaces with label
	k8sSecrets, err := s.KubeClient.Core().Secrets(api.NamespaceAll).List(api.ListOptions{})
	if err != nil {
		return nil, "", err
	}

	// Filter out the secrets that are not router API Key secrets or that do not have the proper secret key
	secrets := []WatchableResource{}

	// Filter secrets that have the APIKeySecret name
	for _, secret := range k8sSecrets.Items {
		if secret.Name == s.Config.APIKeySecret {
			secrets = append(secrets, s.ConvertToModel(&secret))
		}
	}

	return secrets, k8sSecrets.ListMeta.ResourceVersion, nil
}

/*
ConvertToModel converts an *api.Secret k8s secret to a WatchableResource
*/
func (s SecretWatchableSet) ConvertToModel(in interface{}) WatchableResource {
	k8Secret := in.(*api.Secret)
	secret := &Secret{
		Namespace: k8Secret.Namespace,
		Fields:    k8Secret.Data,
	}

	if routingKey, ok := k8Secret.Data[s.Config.APIKeySecretDataField]; ok {
		secret.RoutingKey = &routingKey
	}

	// Pre calculdate hash
	secret.hash = calculateSecretHash(secret)

	return secret
}

/*
Watchable tests where the *api.Secret inputed has the Name of of the configured APIKeySecret
*/
func (s SecretWatchableSet) Watchable(in interface{}) bool {
	k8Secret := in.(*api.Secret)
	if k8Secret.Name != s.Config.APIKeySecret {
		return false
	}
	return true
}

/*
CacheAdd adds Secret to the caches Secret bucket
*/
func (s SecretWatchableSet) CacheAdd(cache *Cache, item WatchableResource) {
	secret := item.(*Secret)
	cache.Secrets[item.ID()] = secret
}

/*
CacheRemove removes the Secret using the id given from the Cache's Secrets bucket
*/
func (s SecretWatchableSet) CacheRemove(cache *Cache, id string) {
	delete(cache.Secrets, id)
}

/*
CacheCompare compares the given Secret with the Secret in the cache, if equal returns true otherwise returns false. If cache value does not exist return false.
*/
func (s SecretWatchableSet) CacheCompare(cache *Cache, newItem WatchableResource) bool {
	item, ok := cache.Secrets[newItem.ID()]
	if !ok {
		return false
	}
	return item.Hash() == newItem.Hash()
}

/*
IDFromObject returns the Namespace name from the *api.Secret
*/
func (s SecretWatchableSet) IDFromObject(in interface{}) string {
	secret := in.(*api.Secret)
	return secret.Namespace
}
