/*
Copyright © 2016 Apigee Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package router

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/30x/k8s-router/kubernetes"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

// config is set in pods_test.go

func init() {
	log.SetOutput(ioutil.Discard)
}

/*
Test for github.com/30x/k8s-router/router/secrets#GetRouterSecretList
*/
func TestGetRouterSecretList(t *testing.T) {
	kubeClient, err := kubernetes.GetClient()

	if err != nil {
		t.Fatalf("Failed to create k8s client: %v.", err)
	}

	secretList, err := GetRouterSecretList(config, kubeClient)

	if err != nil {
		t.Fatalf("Failed to get the router secrets: %v.", err)
	}

	for _, secret := range secretList.Items {
		if secret.Name != config.APIKeySecret {
			t.Fatalf("Every secret should have a %s name", config.APIKeySecret)
		}
	}
}

/*
Test for github.com/30x/k8s-router/router/secrets#UpdateSecretCacheForEvents
*/
func TestUpdateSecretCacheForEvents(t *testing.T) {
	apiKeyStr := "API-Key"
	apiKey := []byte(apiKeyStr)
	cache := make(map[string][]byte)
	namespace := "my-namespace"

	addedSecret := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}
	modifiedSecretNoRestart := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
			"new-key": []byte("New-API-Key"),
		},
	}
	modifiedSecretRestart := &api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": []byte("Updated-API-Key"),
		},
	}

	// Test add event
	needsRestart := UpdateSecretCacheForEvents(config, cache, []watch.Event{
		watch.Event{
			Type:   watch.Added,
			Object: addedSecret,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	} else if _, ok := cache[namespace]; !ok {
		t.Fatal("Cache should reflect the added secret")
	}

	// Test modify event with unchanged api-key
	needsRestart = UpdateSecretCacheForEvents(config, cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedSecretNoRestart,
		},
	})

	if needsRestart {
		t.Fatal("Server should not require a restart")
	}

	// Test modify event with changed api-key
	needsRestart = UpdateSecretCacheForEvents(config, cache, []watch.Event{
		watch.Event{
			Type:   watch.Modified,
			Object: modifiedSecretRestart,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	}

	if apiKeyStr == string(cache[namespace][:]) {
		t.Fatal("Cache should have the updated secret")
	}

	// Test delete event
	needsRestart = UpdateSecretCacheForEvents(config, cache, []watch.Event{
		watch.Event{
			Type:   watch.Deleted,
			Object: addedSecret,
		},
	})

	if !needsRestart {
		t.Fatal("Server should require a restart")
	} else if _, ok := cache[namespace]; ok {
		t.Fatal("Cache should not have the deleted secret")
	}
}
