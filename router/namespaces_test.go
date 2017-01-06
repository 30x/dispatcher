package router

import (
	"encoding/json"
	api "k8s.io/client-go/pkg/api/v1"
	"strings"
	"testing"
)

func init() {
	// Config setup in ./secrets_test.go
}

func genHostsJSON(hosts string) string {
	obj := make(map[string]HostOptions)
	for _, host := range strings.Split(hosts, " ") {
		obj[host] = HostOptions{}
	}

	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return string(b)
}

func genNamespace(name, org, env, hosts string) Namespace {
	k8sNs := genK8sNamespace(name, org, env, hosts)
	obj := make(map[string]HostOptions)
	for _, host := range strings.Split(hosts, " ") {
		obj[host] = HostOptions{}
	}
	return Namespace{
		Name:         name,
		Organization: org,
		Environment:  env,
		Hosts:        obj,
		hash:         calculateNamespaceHash(config, &k8sNs),
	}
}

func genK8sNamespace(name, org, env, hosts string) api.Namespace {
	return api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				config.NamespaceOrgAnnotation:   org,
				config.NamespaceEnvAnnotation:   env,
				config.NamespaceHostsAnnotation: hosts,
			},
			Labels: map[string]string{
				"github.com/30x.dispatcher.ns": "true",
			},
		},
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#Namespace.Id
*/
func TestNamespaceId(t *testing.T) {
	namespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	if namespace.ID() != "my-namespace" {
		t.Fatalf("Namespace Id() should be \"my-namespace\" but was %s.", namespace.ID())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#Namespace.Hash
*/
func TestNamespaceHash(t *testing.T) {
	namespace1 := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace2 := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace3 := genNamespace("my-namespace", "diff-org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace4 := genNamespace("my-namespace", "org", "diff-test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace5 := genNamespace("my-namespace", "org", "test", genHostsJSON("org2-test.ex.net api2.ex.net"))

	if namespace1.Hash() != 9602405720185102016 {
		t.Fatalf("Namespace Hash() should match 9602405720185102016 but was %d", namespace1.Hash())
	}

	if namespace1.Hash() != namespace2.Hash() {
		t.Fatalf("Namespace Hash() should match namespace1's hash %d != %d", namespace1.Hash(), namespace2.Hash())
	}

	if namespace1.Hash() == namespace3.Hash() {
		t.Fatalf("Namespace Hash() should not match namespace3's hash %d == %d wen org is different", namespace1.Hash(), namespace3.Hash())
	}
	if namespace1.Hash() == namespace4.Hash() {
		t.Fatalf("Namespace Hash() should not match namespace4's hash %d == %d wen env is different", namespace1.Hash(), namespace4.Hash())
	}
	if namespace1.Hash() == namespace5.Hash() {
		t.Fatalf("Namespace Hash() should not match namespace5's hash %d == %d wen hosts are different", namespace1.Hash(), namespace5.Hash())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.ConvertToModel
*/
func TestNamespaceConvertToModel(t *testing.T) {
	k8sNamespace := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	set := NamespaceWatchableSet{Config: config}
	item := set.ConvertToModel(&k8sNamespace)

	if item.ID() != "my-namespace" {
		t.Fatalf("Namespace Id() should match \"my-namespace\" but was %s", item.ID())
	}

	ns := item.(*Namespace)
	if ns.Organization != "org" {
		t.Fatalf("Namespace Organization should match \"org\" but was %s", ns.Organization)
	}

	if ns.Environment != "test" {
		t.Fatalf("Namespace Environment should match \"test\" but was %s", ns.Environment)
	}

	if len(ns.Hosts) != 2 {
		t.Fatalf("Namespace Hosts should have 2 hosts but had %d", len(ns.Hosts))
	}

	if _, ok := ns.Hosts["org-test.ex.net"]; !ok {
		t.Fatalf("Namespace Host should have \"org-test.ex.net\"")
	}

	if _, ok := ns.Hosts["api.ex.net"]; !ok {
		t.Fatalf("Namespace Host should have \"api.ex.net\"")
	}

	k8sNamespace2 := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net invalid#>.host api.ex.net"))
	item2 := set.ConvertToModel(&k8sNamespace2)
	ns2 := item2.(*Namespace)
	if len(ns2.Hosts) != 2 {
		t.Fatalf("Namespace Hosts should only contain valid hosts and have a length of 2 but was %d", len(ns2.Hosts))
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.IDFromObject
*/
func TestNamespaceIDFromObject(t *testing.T) {
	k8sNamespace := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	set := NamespaceWatchableSet{Config: config}
	if set.IDFromObject(&k8sNamespace) != "my-namespace" {
		t.Fatalf("IDFromObject on k8s object should return \"my-namespace\" but was %s", set.IDFromObject(&k8sNamespace))
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.Watchable
*/
func TestNamespaceWatchable(t *testing.T) {
	k8sNamespace := genK8sNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	k8sNamespaceNon := api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: "my-namespace",
			Annotations: map[string]string{
				config.NamespaceOrgAnnotation:   "org",
				config.NamespaceEnvAnnotation:   "test",
				config.NamespaceHostsAnnotation: genHostsJSON("org-test.ex.net api.ex.net"),
			},
			Labels: map[string]string{
				"github.com/30x.dispatcher.ns": "false",
			},
		},
	}

	set := NamespaceWatchableSet{Config: config}
	if set.Watchable(&k8sNamespace) != true {
		t.Fatalf("k8sNamespace should be watchable")
	}

	if set.Watchable(&k8sNamespaceNon) == true {
		t.Fatalf("k8sNamespaceNon should not be watchable")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheAdd
*/
func TestNamespaceCacheAdd(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	namespace1 := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace2 := genNamespace("my-namespace2", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	set.CacheAdd(cache, &namespace1)
	set.CacheAdd(cache, &namespace2)

	testNamespace1, ok := cache.Namespaces["my-namespace"]
	if !ok {
		t.Fatalf("Test namespace 1 not in cache")
	}

	testNamespace2, ok := cache.Namespaces["my-namespace2"]
	if !ok {
		t.Fatalf("Test namespace 2 not in cache")
	}

	if testNamespace1 != &namespace1 {
		t.Fatalf("Test namespace 1 should be in cache for my-namespace key")
	}

	if testNamespace2 != &namespace2 {
		t.Fatalf("Test namespace 2 should be in cache for my-namespace2 key")
	}

	namespace3 := genNamespace("my-namespace", "org", "test", genHostsJSON("orgdiff-test.ex.net api2.ex.net"))
	set.CacheAdd(cache, &namespace3)
	testNamespace3, ok := cache.Namespaces["my-namespace"]
	if !ok {
		t.Fatalf("Test namespace with key my-namespace not in cache")
	}
	if testNamespace3 != &namespace3 {
		t.Fatalf("Test namespace with key my-namespace does not equal the updated value %v != %v", testNamespace3, namespace3)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheRemove
*/
func TestNamespacesCacheRemove(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	namespace := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	cache.Namespaces[namespace.ID()] = &namespace

	set.CacheRemove(cache, namespace.ID())

	_, ok := cache.Namespaces[namespace.ID()]
	if ok == true {
		t.Fatalf("Namespace should be removed from cache after CacheRemove")
	}

	set.CacheRemove(cache, "not-an-id")
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheCompare
*/
func TestNamespacesCacheCompare(t *testing.T) {
	cache := NewCache()
	set := NamespaceWatchableSet{Config: config}
	namespace1 := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace2 := genNamespace("my-namespace", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace3 := genNamespace("my-namespace", "org2", "test2", genHostsJSON("org-test.ex.net api.ex.net"))
	namespace4 := genNamespace("my-namespace2", "org", "test", genHostsJSON("org-test.ex.net api.ex.net"))

	cache.Namespaces[namespace1.ID()] = &namespace1
	if set.CacheCompare(cache, &namespace2) != true {
		t.Fatalf("Namespace2 should match secret1 that is in cache")
	}

	if set.CacheCompare(cache, &namespace3) != false {
		t.Fatalf("Namespace3 should not match secret1 that is in cache")
	}

	if set.CacheCompare(cache, &namespace4) != false {
		t.Fatalf("Namespace4 should not match anything that is in cache, namespace not added.")
	}
}
