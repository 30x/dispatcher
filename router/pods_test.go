package router

import (
	"encoding/json"
	api "k8s.io/client-go/pkg/api/v1"
	"strconv"
	"testing"
)

func path(basePath, port, targetPath string) pathAnnotation {
	p := pathAnnotation{basePath, port, nil}
	if targetPath != "" {
		p.TargetPath = &targetPath
	}
	return p
}

func init() {
	// Config setup in ./secrets_test.go
}

func genRoutes(routes ...pathAnnotation) string {
	b, err := json.Marshal(routes)
	if err != nil {
		panic(err)
	}

	return string(b)

}

func genPod(name, paths, ip string, status api.PodPhase, containerPorts []string) *PodWithRoutes {
	set := PodWatchableSet{Config: config}
	item := set.ConvertToModel(genK8sPod(name, paths, ip, status, containerPorts))
	return item.(*PodWithRoutes)
}

func genK8sPod(name, paths, ip string, status api.PodPhase, containerPorts []string) *api.Pod {
	pod := api.Pod{
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: "my-namespace",
			Annotations: map[string]string{
				config.PodsPathsAnnotation: paths,
			},
			Labels: map[string]string{
				"github.com/30x.dispatcher.routable": "true",
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{},
				},
			},
		},
		Status: api.PodStatus{
			Phase: status,
			PodIP: ip,
		},
	}

	for _, port := range containerPorts {
		intPort, _ := strconv.Atoi(port)
		pod.Spec.Containers[0].Ports = append(pod.Spec.Containers[0].Ports, api.ContainerPort{
			ContainerPort: int32(intPort),
		})
	}

	return &pod
}

func validateRoutes(t *testing.T, desc string, expected, actual []*Route) {
	aCount := 0
	eCount := 0

	if actual != nil {
		aCount = len(actual)
	}

	if expected != nil {
		eCount = len(expected)
	}

	// First check that we have the proper number of routes
	if aCount != eCount {
		t.Fatalf("Expected %d routes but found %d routes: %s\n", eCount, aCount, desc)
	}

	// Validate each route positionally
	find := func(items []*Route, item *Route) *Route {
		var route *Route

		for _, cRoute := range items {
			if item.Incoming.Path == cRoute.Incoming.Path &&
				item.Outgoing.IP == cRoute.Outgoing.IP &&
				item.Outgoing.Port == cRoute.Outgoing.Port &&
				item.Outgoing.TargetPath == cRoute.Outgoing.TargetPath {
				route = cRoute

				break
			}
		}

		return route
	}

	for _, route := range expected {
		if find(actual, route) == nil {
			t.Fatalf("Unable to find route (%s): %s\n", route, desc)
		}
	}
}

/*
Test for github.com/30x/k8s-router/router/pods#GetRoutes where the pod is not running
*/
func TestGetRoutesNotRunning(t *testing.T) {
	validateRoutes(t, "pod not running", []*Route{}, GetRoutes(config, &api.Pod{
		Status: api.PodStatus{
			Phase: api.PodPending,
		},
	}))
}

/*
Test for github.com/30x/k8s-router/router/pods#GetRoutes where the pod is running but does not have an IP
*/
func TestGetRoutesRunningWithoutIP(t *testing.T) {
	validateRoutes(t, "pod does not have an IP", []*Route{}, GetRoutes(config, &api.Pod{
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-router/router/pods#GetRoutes where the pod has an invalid port value in the routingPaths annotation
*/
func TestGetRoutesInvalidPublicPathsPort(t *testing.T) {
	// Not a valid integer
	validateRoutes(t, "pod has an invalid routingPaths port (invalid integer)", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "abcdef", "")),
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// Port is less than 0
	validateRoutes(t, "pod has an invalid routingPaths port (port < 0)", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "-1", "")),
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// Port is greater than 65535
	validateRoutes(t, "pod has an invalid routingPaths port (port > 65536)", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "77777", "")),
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// Port is not an exposed container port
	validateRoutes(t, "pod has an invalid routingPaths port, is not an exposed container port", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				"routingHosts":             "test.github.com",
				config.PodsPathsAnnotation: genRoutes(path("/", "81", "")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(80),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-router/router/pods#GetRoutes where the pod has an invalid path value in the routingPaths annotation
*/
func TestGetRoutesInvalidPublicPathsPath(t *testing.T) {
	// "%ZZ" is not a valid path segment
	validateRoutes(t, "pod has an invalid routingPaths path", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/people/%ZZ", "3000", "")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))

	// "%ZZ" is not a valid path for targetPath segment
	validateRoutes(t, "pod has an invalid routingPaths path", []*Route{}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "3000", "/people/%ZZ")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}))
}

/*
Test for github.com/30x/k8s-router/router/pods#GetRoutes where the pod has no routingPaths annotation
*/
func TestGetRoutesValidPods(t *testing.T) {
	ip := "10.244.1.17"
	path1 := "/"
	path2 := "/admin/"
	targetPath1 := "/users/"
	port1 := "3000"
	port2 := "3001"

	// A single host and path
	validateRoutes(t, "single host and path", []*Route{
		&Route{
			Incoming: &Incoming{
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
	}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path(path1, port1, "")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

	// A single host and path with tagetPath
	validateRoutes(t, "single host and path", []*Route{
		&Route{
			Incoming: &Incoming{
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:         ip,
				Port:       port1,
				TargetPath: targetPath1,
			},
		},
	}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path(path1, port1, targetPath1)),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

	// A single host and multiple paths
	validateRoutes(t, "single host and multiple paths", []*Route{
		&Route{
			Incoming: &Incoming{
				Path: path1,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port1,
			},
		},
		&Route{
			Incoming: &Incoming{
				Path: path2,
			},
			Outgoing: &Outgoing{
				IP:   ip,
				Port: port2,
			},
		},
	}, GetRoutes(config, &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path(path1, port1, ""), path(path2, port2, "")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
						api.ContainerPort{
							ContainerPort: int32(3001),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
			PodIP: ip,
		},
	}))

}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWithRoute.Id
*/
func TestPodWithRouteId(t *testing.T) {
	pod := PodWithRoutes{Name: "some-pod"}
	if pod.ID() != "some-pod" {
		t.Fatalf("Pod Id() should be \"some-pod\" but was %s.", pod.ID())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWithRoute.Hash
*/
func TestPodWithRouteHash(t *testing.T) {
	ip := "10.244.1.17"
	name1 := "some-pod"
	path1 := "/"
	port1 := "3000"

	podA := genPod(name1, genRoutes(path(path1, port1, "")), ip, api.PodRunning, []string{port1})
	// Same as 1
	podB := genPod(name1, genRoutes(path(path1, port1, "")), ip, api.PodRunning, []string{port1})
	if podA.Hash() != podB.Hash() {
		t.Fatalf("Pod's hash do not match hash %d != %d", podA.Hash(), podB.Hash())
	}

	// Differnt name
	podB = genPod("other-name", genRoutes(path(path1, port1, "")), ip, api.PodRunning, []string{port1})
	if podA.Hash() == podB.Hash() {
		t.Fatalf("Pod's hash should not match when name is differnt %d != %d", podA.Hash(), podB.Hash())
	}

	// Different path annotation
	podB = genPod(name1, genRoutes(path("/other/path", port1, "")), ip, api.PodRunning, []string{port1})
	if podA.Hash() == podB.Hash() {
		t.Fatalf("Pod's hash should not match when path annotation is differnt %d != %d", podA.Hash(), podB.Hash())
	}

	// Different ip
	podB = genPod(name1, genRoutes(path(path1, port1, "")), "1.2.3.4", api.PodRunning, []string{port1})
	if podA.Hash() == podB.Hash() {
		t.Fatalf("Pod's hash should not match when ip is differnt %d != %d", podA.Hash(), podB.Hash())
	}

	// Different status
	podB = genPod(name1, genRoutes(path(path1, port1, "")), ip, api.PodPending, []string{port1})
	if podA.Hash() == podB.Hash() {
		t.Fatalf("Pod's hash should not match when status is differnt %d != %d", podA.Hash(), podB.Hash())
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.ConvertToModel
*/
func TestPodWatchableSetConvertToModel(t *testing.T) {
	k8sPod := genK8sPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	set := PodWatchableSet{Config: config}
	item := set.ConvertToModel(k8sPod)

	if item.ID() != "some-pod" {
		t.Fatalf("Pod Id() should match \"some-pod\" but was %s", item.ID())
	}

	pod := item.(*PodWithRoutes)
	if pod.Namespace != "my-namespace" {
		t.Fatalf("Pod Namespace should match \"my-namespace\" but was %s", pod.Namespace)
	}

	if pod.Status != api.PodRunning {
		t.Fatalf("Pod Status should match %v but was %s", api.PodRunning, pod.Status)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.IDFromObject
*/
func TestPodWatchableSetIDFromObject(t *testing.T) {
	k8sPod := genK8sPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	set := PodWatchableSet{Config: config}
	if set.IDFromObject(k8sPod) != "some-pod" {
		t.Fatalf("IDFromObject on k8s object should return \"some-pod\" but was %s", set.IDFromObject(k8sPod))
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.Watchable
*/
func TestPodWatchableSetWatchable(t *testing.T) {
	k8sPod := genK8sPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	// Pod does not have routable label
	k8sPodNon1 := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "3000", "/people/%ZZ")),
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodRunning,
		},
	}

	// Pod is not running
	k8sPodNon2 := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			Annotations: map[string]string{
				config.PodsPathsAnnotation: genRoutes(path("/", "3000", "/people/%ZZ")),
			},
			Labels: map[string]string{
				"github.com/30x.dispatcher.routable": "true",
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				api.Container{
					Ports: []api.ContainerPort{
						api.ContainerPort{
							ContainerPort: int32(3000),
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: api.PodFailed,
		},
	}

	set := PodWatchableSet{Config: config}
	if set.Watchable(k8sPod) != true {
		t.Fatalf("k8sPod should be watchable")
	}

	if set.Watchable(k8sPodNon1) == true {
		t.Fatalf("k8sPod should not be watchable, no label selector")
	}

	if set.Watchable(k8sPodNon2) == true {
		t.Fatalf("k8sPod should not be watchable, not running")
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.CacheAdd
*/
func TestPodWatchableSetCacheAdd(t *testing.T) {
	cache := NewCache()
	set := PodWatchableSet{Config: config}
	pod1 := genPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	pod2 := genPod("some-other-pod", genRoutes(path("/", "3000", "")), "2.2.3.4", api.PodRunning, []string{"3000"})

	set.CacheAdd(cache, pod1)
	set.CacheAdd(cache, pod2)

	testPod1, ok := cache.Pods["some-pod"]
	if !ok {
		t.Fatalf("Test pod 1 not in cache")
	}

	testPod2, ok := cache.Pods["some-other-pod"]
	if !ok {
		t.Fatalf("Test pod 2 not in cache")
	}

	if testPod1 != pod1 {
		t.Fatalf("Test pod 1 should be in cache for some-pod key")
	}

	if testPod2 != pod2 {
		t.Fatalf("Test pod 2 should be in cache for some-other-pod key")
	}

	pod3 := genPod("some-pod", genRoutes(path("/other", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	set.CacheAdd(cache, pod3)
	testPod3, ok := cache.Pods["some-pod"]
	if !ok {
		t.Fatalf("Test pod with key some-pod not in cache")
	}
	if testPod3 != pod3 {
		t.Fatalf("Test pod with key some-pod does not equal the updated value %v != %v", testPod3, pod3)
	}
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.CacheRemove
*/
func TestPodWatchableSetCacheRemove(t *testing.T) {
	cache := NewCache()
	set := PodWatchableSet{Config: config}
	pod := genPod("some-pod", genRoutes(path("/other", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})

	cache.Pods[pod.ID()] = pod

	set.CacheRemove(cache, pod.ID())

	_, ok := cache.Pods[pod.ID()]
	if ok == true {
		t.Fatalf("Pod should be removed from cache after CacheRemove")
	}

	set.CacheRemove(cache, "not-an-id")
}

/*
Test for github.com/30x/dispatcher/pkg/router#PodWatchableSet.CacheCompare
*/
func TestPodWatchableSetCacheCompare(t *testing.T) {
	cache := NewCache()
	set := PodWatchableSet{Config: config}
	pod1 := genPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	pod2 := genPod("some-pod", genRoutes(path("/", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	pod3 := genPod("some-pod", genRoutes(path("/other", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})
	pod4 := genPod("other-pod", genRoutes(path("/other", "3000", "")), "1.2.3.4", api.PodRunning, []string{"3000"})

	cache.Pods[pod1.ID()] = pod1
	if set.CacheCompare(cache, pod2) != true {
		t.Fatalf("Pod2 should match pod that is in cache")
	}

	if set.CacheCompare(cache, pod3) != false {
		t.Fatalf("Pod3 should not match pod in cache with different path annotations.")
	}

	if set.CacheCompare(cache, pod4) != false {
		t.Fatalf("Pod3 should not match anything that is in cache, namespace not added.")
	}
}
