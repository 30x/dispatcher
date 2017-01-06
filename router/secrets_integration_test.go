// +build integration

package router

import (
	kube "github.com/30x/dispatcher/kubernetes"
	"k8s.io/client-go/kubernetes"
	"log"
	"testing"
)

var client *kubernetes.Clientset

func init() {

	// Config setup in ./secrets_test.go

	tmpClient, err := kube.GetClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	}

	client = tmpClient
}

/*
Test for github.com/30x/dispatcher/pkg/router#Secret.Id
*/
func TestSecretsGet(t *testing.T) {
	set := SecretWatchableSet{Config: config, KubeClient: client}
	list, version, err := set.Get()
	if err != nil {
		t.Fatalf("Failed to Get secrets: %v.", err)
	}

	if version == "" {
		t.Fatalf("Version must be set: %v.", version)
	}

	for _, item := range list {
		secret := item.(*Secret)
		if secret.Namespace == "" {
			t.Fatalf("Secret Namespace must be set")
		}
	}
}
