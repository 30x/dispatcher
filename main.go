package main

import (
	kube "github.com/30x/dispatcher/kubernetes"
	"github.com/30x/dispatcher/nginx"
	"github.com/30x/dispatcher/router"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/watch"
	"log"
	"reflect"
	"time"
)

// ResourceWatch tuple for a resource watch set and the k8s watch interface
type ResourceWatch struct {
	Resource router.WatchableResourceSet
	Watcher  watch.Interface
}

// Event Struct to hold the channel index and actual event when all channels are aggregated
type Event struct {
	Chan  int
	Event watch.Event
}

// Time window to capture events before prossing batch
const eventWindow time.Duration = 2000 * time.Millisecond

func printConf(config router.Config) {
	var logInterface func(v reflect.Value, base string)
	logInterface = func(v reflect.Value, base string) {
		for i := 0; i < v.NumField(); i++ {
			switch v.Type().Field(i).Type.Kind() {
			case reflect.Struct:
				logInterface(v.Field(i), v.Type().Field(i).Name+".")
			default:
				log.Printf("    %s%s  =  %v\n", base, v.Type().Field(i).Name, v.Field(i).Interface())
			}
		}
	}

	log.Println("  Using configuration:")
	logInterface(reflect.ValueOf(config), "")
	log.Println("")
}

func initController(config *router.Config, kubeClient *kubernetes.Clientset) (*router.Cache, []*ResourceWatch) {

	// Init cache
	cache := router.NewCache()

	// Create each watchable resource set. Namespaces, Secrets, Pods, etc...
	resourceTypes := []*ResourceWatch{
		&ResourceWatch{router.NamespaceWatchableSet{config, kubeClient}, nil},
		&ResourceWatch{router.SecretWatchableSet{config, kubeClient}, nil},
		&ResourceWatch{router.PodWatchableSet{config, kubeClient}, nil},
	}

	for _, res := range resourceTypes {
		// Grab current resources from api
		resources, version, err := res.Resource.Get()
		if err != nil {
			log.Fatalf("Failed to: %v.", err)
		}

		// Add each resource to it's respective cache
		for _, item := range resources {
			res.Resource.CacheAdd(cache, item)
		}

		// Create watcher for each resource
		watcher, err := res.Resource.Watch(version)
		if err != nil {
			log.Fatalf("Failed to create watcher: %v.", err)
		}

		res.Watcher = watcher
	}

	// Generate the nginx configuration and restart nginx
	nginx.RestartServer(config, nginx.GetConf(config, cache), false)

	return cache, resourceTypes
}

func main() {
	log.Println("Starting the Kubernetes Router")

	// Get configuration object
	config, err := router.ConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid configuration: %v.", err)
	}

	printConf(*config)

	// Create the Kubernetes Client
	kubeClient, err := kube.GetClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	// Don't write nginx conf when not in cluster
	config.Nginx.RunInMockMode = !(kube.RunningInCluster())

	// Start nginx with the default configuration to start nginx as a daemon
	nginx.StartServer(config, nginx.GetConf(config, router.NewCache()))

	// Loop forever
	for {
		// Create the initial cache and watchers
		cache, resourceTypes := initController(config, kubeClient)

		// List of events gathered during window
		events := []Event{}
		// Create done channel that is called if any watchers close
		done := make(chan struct{})
		combinedChannel := make(chan Event)

		// Aggragate all resource types into one channel
		for i, res := range resourceTypes {
			go func(n int, c <-chan watch.Event) {
				for v := range c {
					combinedChannel <- Event{n, v}
				}
				done <- struct{}{}
			}(i, res.Watcher.ResultChan())
		}

		// Keep track of the first event seen and when happened to start the timer of when to stop
		firstEvent := false
		start := time.Now()
		waitTime := eventWindow

		// process events from watchers until channels shutdown
	Process:
		for {
			select {
			case e := <-combinedChannel:
				if !firstEvent {
					// First event seen since timer triggered, start clock now
					firstEvent = true
					start = time.Now()
				} else {
					// Update waitTime from when the first event was seen
					waitTime = eventWindow - time.Since(start)
				}
				// Buffer events to be processed after 2s from the first event
				events = append(events, e)
			case <-time.After(waitTime):
				needsRestart := false
				// Process all events for the event window
				for _, e := range events {
					// If data has changed restart nginx
					if router.ProcessEvent(cache, resourceTypes[e.Chan].Resource, e.Event) {
						needsRestart = true
					}
				}

				//  If nginx needs restart
				if needsRestart {
					log.Println("Nginx needs restart.")
					nginx.RestartServer(config, nginx.GetConf(config, cache), false)
				}

				// Clear events and reset the wait time for the event window
				events = []Event{}
				waitTime = eventWindow
				firstEvent = false
			case <-done:
				// Shutdown all watchers and restart
				for _, res := range resourceTypes {
					res.Watcher.Stop()
				}
				// Break out of processing
				break Process
			}
		}

		log.Println("Watchers exited, restarting.")
	}

}
