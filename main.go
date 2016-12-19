package main

import (
	"fmt"
	"github.com/30x/dispatcher/pkg/kubernetes"
	"github.com/30x/dispatcher/pkg/router"
	api "k8s.io/client-go/pkg/api/v1"
	"log"
	"time"
)

func main() {

	config, err := router.ConfigFromEnv()
	if err != nil {
		log.Fatalf("Invalid configuration: %v.", err)
	}

	kubeClient, err := kubernetes.GetClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	namespaces, err := router.GetNamespaces(config, kubeClient)
	if err != nil {
		log.Fatalf("Failed to get namespaces: %v.", err)
	}

	for i, namespace := range namespaces.Items {
		fmt.Printf("%d - Namespace %s\n", i, namespace.Name)
	}

	for {
		pods, err := kubeClient.Core().Pods("").List(api.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
		time.Sleep(10 * time.Second)
	}

}
