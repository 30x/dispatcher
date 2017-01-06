package router

import (
	"bytes"
	"io/ioutil"
	api "k8s.io/client-go/pkg/api/v1"
	"log"
	"testing"
)

var config *Config

func init() {
	envConfig, err := ConfigFromEnv()
	if err != nil {
		log.Fatalf("Unable to get configuration from environment: %v", err)
	}

	config = envConfig

	log.SetOutput(ioutil.Discard)
}

/*
Test for github.com/30x/dispatcher/pkg/router#Secret.Id
*/
func TestSecretId(t *testing.T) {
	secret := Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	if secret.ID() != "my-namespace" {
		t.Fatalf("Secret Id() should be \"my-namespace\" but was %s.", secret.ID())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#Secret.Hash
*/
func TestSecretHash(t *testing.T) {
	secret1 := Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	secret2 := Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	secret3 := Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x6}}
	secret4 := Secret{Namespace: "my-namespace2", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x6}}
	if secret1.Hash() != 3708964778940489642 {
		t.Fatalf("Secret Hash() should match 3708964778940489642 but was %d", secret1.Hash())
	}

	if secret1.Hash() != secret2.Hash() {
		t.Fatalf("Secret Hash() should match secret2's hash %d != %d", secret1.Hash(), secret2.Hash())
	}

	if secret1.Hash() == secret3.Hash() {
		t.Fatalf("Secret Hash() should not match secret2's hash %d == %d", secret1.Hash(), secret3.Hash())
	}

	if secret3.Hash() != secret4.Hash() {
		t.Fatalf("Secret3 Hash() should match secret4's hash %d != %d", secret3.Hash(), secret4.Hash())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.ConvertToModel
*/
func TestSecretsConvertToModel(t *testing.T) {
	apiKey := []byte("API-Key")
	k8sSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			config.APIKeySecretDataField: apiKey,
		},
	}

	secrets := SecretWatchableSet{Config: config}
	item := secrets.ConvertToModel(&k8sSecret)

	if item.ID() != "my-namespace" {
		t.Fatalf("Secret Id() should match \"my-namespace\" but was %s", item.ID())
	}

	secret := item.(*Secret)
	if bytes.Compare(secret.Data, apiKey) != 0 {
		t.Fatalf("Secret Data should match apiKey %v != %v", secret.Data, apiKey)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.IDFromObject
*/
func TestSecretsIDFromObject(t *testing.T) {
	apiKey := []byte("API-Key")
	k8sSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"api-key": apiKey,
		},
	}

	secrets := SecretWatchableSet{Config: config}
	if secrets.IDFromObject(&k8sSecret) != "my-namespace" {
		t.Fatalf("IDFromObject on k8s object should return \"my-namespace\" but was %s", secrets.IDFromObject(&k8sSecret))
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.Watchable
*/
func TestSecretsWatchable(t *testing.T) {
	apiKey := []byte("API-Key")
	k8sSecret := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      config.APIKeySecret,
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			config.APIKeySecretDataField: apiKey,
		},
	}

	k8sSecretNon := api.Secret{
		ObjectMeta: api.ObjectMeta{
			Name:      "no-routing",
			Namespace: "my-namespace",
		},
		Data: map[string][]byte{
			"not-api-key": apiKey,
		},
	}

	secrets := SecretWatchableSet{Config: config}
	if secrets.Watchable(&k8sSecret) != true {
		t.Fatalf("k8sSecret should be watchable")
	}

	if secrets.Watchable(&k8sSecretNon) == true {
		t.Fatalf("k8sSecretNon should not be watchable")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheAdd
*/
func TestSecretsCacheAdd(t *testing.T) {
	cache := NewCache()
	secrets := SecretWatchableSet{Config: config}
	secret1 := &Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	secret2 := &Secret{Namespace: "my-namespace2", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x6}}

	secrets.CacheAdd(cache, secret1)
	secrets.CacheAdd(cache, secret2)

	testSecret1, ok := cache.Secrets["my-namespace"]
	if !ok {
		t.Fatalf("Test secret 1 not in cache")
	}

	testSecret2, ok := cache.Secrets["my-namespace2"]
	if !ok {
		t.Fatalf("Test secret 2 not in cache")
	}

	if testSecret1 != secret1 {
		t.Fatalf("Test secret 1 should be in cache for my-namespace key")
	}

	if testSecret2 != secret2 {
		t.Fatalf("Test secret 2 should be in cache for my-namespace2 key")
	}

	secret3 := &Secret{Namespace: "my-namespace", Data: []byte{0x2, 0x2, 0x2, 0x2, 0x2}}
	secrets.CacheAdd(cache, secret3)
	testSecret3, ok := cache.Secrets["my-namespace"]
	if !ok {
		t.Fatalf("Test secret with key my-namespace not in cache")
	}
	if testSecret3 != secret3 {
		t.Fatalf("Test secret with key my-namespace does not equal the updated value %v != %v", testSecret3, secret3)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheRemove
*/
func TestSecretsCacheRemove(t *testing.T) {
	cache := NewCache()
	set := SecretWatchableSet{Config: config}
	secret := &Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}

	cache.Secrets[secret.ID()] = secret

	set.CacheRemove(cache, secret.ID())

	_, ok := cache.Secrets[secret.ID()]
	if ok == true {
		t.Fatalf("Secret should be removed from cache after CacheRemove")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.CacheCompare
*/
func TestSecretsCacheCompare(t *testing.T) {
	cache := NewCache()
	set := SecretWatchableSet{Config: config}
	secret1 := &Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	secret2 := &Secret{Namespace: "my-namespace", Data: []byte{0x1, 0x2, 0x3, 0x4, 0x5}}
	secret3 := &Secret{Namespace: "my-namespace", Data: []byte{0x6, 0x7, 0x8, 0x9, 0x0}}
	secret4 := &Secret{Namespace: "my-namespace2", Data: []byte{0x6, 0x7, 0x8, 0x9, 0x0}}

	cache.Secrets[secret1.ID()] = secret1
	if set.CacheCompare(cache, secret2) != true {
		t.Fatalf("Secret2 should match secret1 that is in cache")
	}

	if set.CacheCompare(cache, secret3) != false {
		t.Fatalf("Secret3 should not match secret1 that is in cache")
	}

	if set.CacheCompare(cache, secret4) != false {
		t.Fatalf("Secret4 should not match with secret that is not added to cache")
	}
}
