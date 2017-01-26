package router

import (
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
	"testing"
)

func init() {

	// Config setup in ./secrets_test.go
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource added
*/
func TestProcessEventResourceAdded(t *testing.T) {
	ns := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	set := NamespaceWatchableSet{Config: config}
	cache := NewCache()
	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Added,
		Object: &ns,
	})

	if !needsRestart {
		t.Fatal("adding resouce should trigger restart")
	}

	_, ok := cache.Namespaces["my-namespace"]
	if !ok {
		t.Fatal("resouce should be added to cache")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource deleted
*/
func TestProcessEventResourceDeleted(t *testing.T) {
	cache := NewCache()
	tmpNamespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	cache.Namespaces["my-namespace"] = &tmpNamespace

	ns := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	set := NamespaceWatchableSet{Config: config}
	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Deleted,
		Object: &ns,
	})

	if !needsRestart {
		t.Fatal("deleting resouce should trigger restart")
	}

	_, ok := cache.Namespaces["my-namespace"]
	if ok {
		t.Fatal("resouce should be removed from cache")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource modified unchanged
*/
func TestProcessEventResourceModifiedUnChanged(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	tmpNamespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	cache.Namespaces["my-namespace"] = &tmpNamespace

	ns := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Modified,
		Object: &ns,
	})

	if needsRestart {
		t.Fatal("modifing a resource that doesn't change dispatcher model should not restart")
	}

	tmp, ok := cache.Namespaces["my-namespace"]
	if !ok {
		t.Fatal("resouce should not be removed from cache")
	}

	if tmp != &tmpNamespace {
		t.Fatal("resouce in cache should not change")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource modified changed
*/
func TestProcessEventResourceModifiedChanged(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	tmpNamespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	cache.Namespaces["my-namespace"] = &tmpNamespace

	ns := genK8sNamespace("my-namespace", "org2", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Modified,
		Object: &ns,
	})

	if !needsRestart {
		t.Fatal("modifing a resource that changes dispatcher model should restart")
	}

	tmp, ok := cache.Namespaces["my-namespace"]
	if !ok {
		t.Fatal("resouce should not be removed from cache")
	}

	if tmp == &tmpNamespace {
		t.Fatal("resouce in cache should not change")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource modified changed to unwatchable
*/
func TestProcessEventResourceModifiedChangedUnWatchable(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	tmpNamespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	cache.Namespaces["my-namespace"] = &tmpNamespace

	ns := api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: "my-namespace",
			Annotations: map[string]string{
				config.NamespaceHostsAnnotation: genHostsJSON("org-test.ex.net api.ex.net"),
			},
			Labels: map[string]string{
				"github.com/30x.dispatcher.routable": "false",
				config.NamespaceOrgLabel:             "org",
				config.NamespaceEnvLabel:             "test",
			},
		},
	}

	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Modified,
		Object: &ns,
	})

	if !needsRestart {
		t.Fatal("modifing a resource to unwatchable should restart")
	}

	_, ok := cache.Namespaces["my-namespace"]
	if ok {
		t.Fatal("resouce should be removed from cache")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#ProcessEvent - Resource added that is not watchable
*/
func TestProcessEventResourceAddedNonWatchaable(t *testing.T) {
	cache := NewCache()
	set := SecretWatchableSet{Config: config}
	k8sSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      "not-the-right-secret",
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": []byte("API-Key"),
		},
	}

	needsRestart := ProcessEvent(cache, set, watch.Event{
		Type:   watch.Added,
		Object: &k8sSecret,
	})

	if needsRestart {
		t.Fatal("resource added that is not watchable should not trigger restart")
	}

	_, ok := cache.Secrets["my-namespace"]
	if ok {
		t.Fatal("new resource should not be in cache")
	}

}
