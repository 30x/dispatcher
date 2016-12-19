package router

import (
	"k8s.io/client-go/pkg/api"
)

/*
Cache is the structure containing the router API Keys and the routable pods cache and namespaces
*/
type Cache struct {
	Namespaces map[string]*Namespace
	Pods       map[string]*PodWithRoutes
	Secrets    map[string][]byte
}

/*
Namespace describes the information stored on the k8s namespace object for routing
*/
type Namespace struct {
	Name  string
	Hosts []string
}

/*
PodWithRoutes contains a pod and its routes
*/
type PodWithRoutes struct {
	Name      string
	Namespace *Namespace
	Status    api.PodPhase
	// Hash of annotation to quickly compare changes
	AnnotationHash uint64
	Routes         []*Route
}

/*
Route describes the incoming route matching details and the outgoing proxy backend details
*/
type Route struct {
	Incoming *Incoming
	Outgoing *Outgoing
}

/*
Incoming describes the information required to route an incoming request
*/
type Incoming struct {
	Path string
}

/*
Outgoing describes the information required to proxy to a backend
*/
type Outgoing struct {
	IP         string
	Port       string
	TargetPath string
}
