// +build integration

package router

import (
	"testing"
)

func init() {

	// Config setup in ./secrets_test.go
	// client setup in ./secrets_integration_test.go
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.Get
*/
func TestNamespaceWatchableSetGet(t *testing.T) {
	set := NamespaceWatchableSet{Config: config, KubeClient: client}
	_, version, err := set.Get()
	if err != nil {
		t.Fatalf("Failed to Get secrets: %v.", err)
	}

	if version == "" {
		t.Fatalf("Version must be set: %v.", version)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#NamespaceWatchableSet.Watch
*/
func TestNamespaceWatchableSetWatch(t *testing.T) {
	set := NamespaceWatchableSet{Config: config, KubeClient: client}
	_, err := set.Watch("")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v.", err)
	}
}
