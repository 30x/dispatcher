package router

import (
	"hash/fnv"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
	"log"
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
	Namespace string
	Data      []byte
}

/*
Id returns the namespace name
*/
func (s Secret) Id() string {
	return s.Namespace
}

/*
Hash returns a fnv hasve of the Secret Data
*/
func (s Secret) Hash() uint64 {
	h := fnv.New64()
	h.Write(s.Data)
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
			_, ok := secret.Data[s.Config.APIKeySecretDataField]
			if ok {
				secrets = append(secrets, s.ConvertToModel(&secret))
			} else {
				log.Printf("    Router secret for namespace (%s) is not usable: Missing '%s' key\n", secret.Namespace, s.Config.APIKeySecretDataField)
			}
		}
	}

	return secrets, k8sSecrets.ListMeta.ResourceVersion, nil
}

/*
ConvertToModel converts an *api.Secret k8s secret to a WatchableResource
*/
func (s SecretWatchableSet) ConvertToModel(in interface{}) WatchableResource {
	k8Secret := in.(*api.Secret)
	secret := Secret{
		Namespace: k8Secret.Namespace,
		Data:      k8Secret.Data[s.Config.APIKeySecretDataField],
	}
	return secret
}

/*
Watchable tests where the *api.Secret inputed has the Name of of the configured APIKeySecret
*/
func (s SecretWatchableSet) Watchable(in interface{}) bool {
	k8Secret := in.(*api.Secret)
	return k8Secret.Name == s.Config.APIKeySecret
}

/*
CacheAdd adds Secret to the caches Secret bucket
*/
func (s SecretWatchableSet) CacheAdd(cache *Cache, item WatchableResource) {
	secret := item.(Secret)
	cache.Secrets[item.Id()] = &secret
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
	item, ok := cache.Secrets[newItem.Id()]
	if !ok {
		return false
	}
	return item.Hash() == newItem.Hash()
}

/*
IdFromObject returns the Namespace name from the *api.Secret
*/
func (s SecretWatchableSet) IDFromObject(in interface{}) string {
	secret := in.(*api.Secret)
	return secret.Namespace
}
