package main

import (
	kube "github.com/30x/dispatcher/pkg/kubernetes"
	"github.com/30x/dispatcher/pkg/router"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
	"log"
	"time"
)

type controllerData struct {
	cache            *router.Cache
	namespaceWatcher watch.Interface
	podWatcher       watch.Interface
	secretsWatcher   watch.Interface
}

func initController(config *router.Config, kubeClient *kubernetes.Clientset) controllerData {
	// Init cache
	cache := &router.Cache{
		Namespaces: make(map[string]*router.Namespace),
		Pods:       make(map[string]*router.PodWithRoutes),
		Secrets:    make(map[string][]byte),
	}

	log.Println("Searching for routable namespaces")

	// Query the initial list of Namespaces
	namespaces, err := router.GetNamespaces(config, kubeClient)
	if err != nil {
		log.Fatalf("Failed to query the initial list of namespaces: %v.", err)
	}

	log.Printf("  Namespaces found: %d", len(namespaces.Items))

	// Turn the namespaces into a map based on the namespaces's name
	for i, ns := range namespaces.Items {
		cache.Namespaces[ns.Name] = router.ConvertNamespaceToModel(config, &(namespaces.Items[i]))
	}

	// Get the list options so we can create the watch
	namespacesWatchOptions := api.ListOptions{
		LabelSelector:   config.NamespaceRoutableLabelSelector,
		ResourceVersion: namespaces.ListMeta.ResourceVersion,
	}

	// Create a watcher to be notified of Pod events
	namespaceWatcher, err := kubeClient.Core().Namespaces().Watch(namespacesWatchOptions)

	if err != nil {
		log.Fatalf("Failed to create namespace watcher: %v.", err)
	}

	return controllerData{
		cache:            cache,
		namespaceWatcher: namespaceWatcher,
	}
}

func main() {
	log.Println("Starting the Kubernetes Router")

	// Get configuration object
	config, err := router.ConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid configuration: %v.", err)
	}

	// Create the Kubernetes Client
	kubeClient, err := kube.GetClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	// Create the initial cache and watchers
	controllerData := initController(config, kubeClient)

	// Loop forever
	for {
		var namespaceEvents []watch.Event
		var podEvents []watch.Event
		var secretEvents []watch.Event

		// Get a 2 seconds window worth of events
		for {
			doRestart := false
			doStop := false

			select {
			case event, ok := <-controllerData.namespaceWatcher.ResultChan():
				if !ok {
					log.Println("Kubernetes closed the namespace watcher, restarting")
					doRestart = true
				} else {
					namespaceEvents = append(namespaceEvents, event)
				}

				// TODO: Rewrite to start the two seconds after the first post-restart event is seen
			case <-time.After(2 * time.Second):
				doStop = true
			}

			if doStop {
				break
			} else if doRestart {
				controllerData.namespaceWatcher.Stop()

				controllerData = initController(config, kubeClient)
			}
		}

		needsRestart := false

		if len(namespaceEvents) > 0 {
			log.Printf("%d namespace events found", len(namespaceEvents))

			// Update the cache based on the events and check if the server needs to be restarted
			//needsRestart = router.UpdatePodCacheForEvents(config, cache.Namespaces, namespaceEvents)
		}

		// Wrapped in an if/else to limit logging
		if len(namespaceEvents) > 0 || len(podEvents) > 0 || len(secretEvents) > 0 {
			if needsRestart {
				log.Println("  Requires nginx restart: yes")

				// Restart nginx
				// nginx.RestartServer(nginx.GetConf(config, cache), false)
			} else {
				log.Println("  Requires nginx restart: no")
			}
		}
	}

}
