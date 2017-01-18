# Overview

The purpose of this project is to provide a name and path based router for Kubernetes. It started out as an ingress
controller but has since been repurposed to allow for both ingress and other types of routing via to its
configurability.  From an ingress perspective, this router does things a little different than your typical 
[Kubernetes Ingress controller](http://kubernetes.io/docs/user-guide/ingress/):

* This version does pod-level routing instead of service-level routing
* This version does not use the
[Kubernetes Ingress Resource](http://kubernetes.io/docs/user-guide/ingress/#the-ingress-resource) definitions
and instead uses pod-level annotations to wire things up _(This is partially because the built-in ingress resource
is intended for service-based ingress instead of pod-based ingress.)_
* This version uses Namespace level hostname definitions.

But in the end, you have a routing controller capable of doing routing based on the combination of hostname/IP and path.

# Design

This router is written in Go and is intended to be deployed within a container on Kubernetes.  Upon startup, this router
will find the [Pods](http://kubernetes.io/docs/user-guide/pods/) and [Namespaces](http://kubernetes.io/docs/user-guide/namespaces/)
marked for routing _(using a configurable label selector)_ and the [Secrets](http://kubernetes.io/docs/user-guide/secrets/) 
*(using a configurable location)_ used to secure routing for those pods.  _(For more details on the role secrets play in 
this router, please see the [Security section](#security) of this document.)_ 

### Namespaces

The Namespaces marked for routing are then analyzed to identify wiring information used for routign stored in the Namespace's [annotations](http://kubernetes.io/docs/user-guide/annotations/) and [labels](http://kubernetes.io/docs/user-guide/labels/).

Annotations:
* `github.com/30x.dispatcher.hosts`: Stores host-level configuration for each host associated with Namespace. JSON document, whose 
keys are the actual hostnames and whose values are the host-specific configuration. At this time, there is no extra configuration 
available at the host level.  This is a forward-thinking approach to allow for future enhancements.

```
annotations:
  github.com/30x.dispatcher.hosts: >
    {
      “jeremy-test.apigee.net”: {},
      “thoughtspark.org”: {}
    }
```

Labels:
* `github.com/30x.dispatcher.org`: String representing the Apigee organization assocatated with Namespace.
* `github.com/30x.dispatcher.env`: String representing the Apigee enviorment assocatated with Namespace.

### Pods

The Pods marked for routing are then analyzed to identify the wiring information used for routing stored in the Pod's [annotations](http://kubernetes.io/docs/user-guide/annotations/) and [labels](http://kubernetes.io/docs/user-guide/labels/).

Annotations:
* `github.com/30x.dispatcher.paths`:  The value of this annotation is a JSON array of path configuration objects whose properties are defined below:
  * `basePath`: This is the request path the router uses to identify traffic for a specific application.
  * `containerPort`: This is the port of the container running the application logic handling traffic for the associated path.
  * `targetPath (optional):` This is the path that the target application is written to handle traffic for (If this configuration is omitted, the target request path is untouched.  If this configuration is provided, it is a URI rewrite of the path where the router path is replaced by the target path. For example, if your router path was /foo and your target path was /bar, a request for /foo/1 would be sent to the target as /bar/1)

Labels:
* `github.com/30x.dispatcher.app.name`: String representing the name of the application.
* `github.com/30x.dispatcher.app.rev`: String representing the revision of the application.

Once we've found all Namespaces, Pods, and Secrets that are involved in routing, we generate an nginx configuration file and start
nginx.  At this point, we cache all k8s resources to avoid having to requery the full list each time and instead listen
for update events on each resource type.  Any time a resource event occurs that would have an impact on routing, we regenerate
the nginx configuration and reload it.  _(The idea here was to allow for an initial hit to pull all pods but to then
to use the events for as quick a turnaround as possible.)_  Events are processed in 2 second chunks.

Each Pod can expose one or more services by using one or more entries in the `github.com/30x.dispatcher.paths` annotation. All of the
paths exposed via `github.com/30x.dispatcher.paths` are exposed for each of the hosts listed in the `github.com/30x.dispatcher.hosts` 
annotation of its corresponding Namespace.  _(So if you have a traffic Hosts of `host1 host2` and a `github.com/30x.dispatcher.path` of 
`[{"basePath": "/", "containerPort": "80"},{"basePath": "/nodejs", "containerPort": "3000"}]`, you would have 4 separate nginx location
 blocks: `host1/ -> {PodIP}:80`, `host2/ -> {PodIP}:80`, `host1/nodejs -> {PodIP}:3000` and `host2/nodejs -> {PodIP}:3000` 
 Right now there is no way to associate specific paths to specific hosts but it may be something we support in the future.)_

# Configuration

All of the touch points for this router are configurable via environment variables:

General Configuration:
* `API_KEY_SECRET_NAME`: This is the Secret name of the API Key to use to secure communication to your Pods. Default: _(`routing`)_
* `API_KEY_SECRET_FIELD`: This is the Secret fieldname of the API Key to use to secure communication to your Pods. Default: _(`api-key`)_
* `APP_NAME_LABEL`: This is the label name used to store the application's name. _(Default: `github.com/30x.dispatcher.app.name`)_
* `APP_REV_LABEL`: This is the label name used to store the application's revision. _(Default: `github.com/30x.dispatcher.app.rev`)_
* `ENV_LABEL`: This is the label name used to store the Namespace's environment. _(Default: `github.com/30x.dispatcher.env`)_
* `HOSTS_ANNOTATION`: This is the annotation name used to store the JSON Document for the configured hosts of a Namespace. _(Default: `github.com/30x.dispatcher.hosts`)_
* `ORG_LABEL`: This is the label name used to store the Namespace's organization. _(Default: `github.com/30x.dispatcher.org`)_
* `PATHS_ANNOTATION`: This is the annotation name used to store the JSON array of routing path configurations for your Pods _(Default: `github.com/30x.dispatcher.paths`)_
* `ROUTABLE_LABEL_SELECTOR`: This is the [label selector](http://kubernetes.io/docs/user-guide/labels/#label-selectors) used to identify Namepaces and Pods that are marked for routing _(Default: `github.com/30x.dispatcher.routable=true`)_

Nginx Configuration:
* `API_KEY_HEADER`: This is the header name used by nginx to identify the API Key used _(Default: `X-ROUTING-API-KEY`)_
* `DEFAULT_LOCATION_RETURN`: Configures nginx to return a status code of proxy to a URI if a request does not match any configured paths in a known host. Can be either a HTTP status code or URI that nginx uses with proxy_pass. _(Default: `404`)_
* `NGINX_ENABLE_HEALTH_CHECKS`: Enables nginx upstream health checks. (Default: `false`)
* `NGINX_MAX_CLIENT_BODY_SIZE`: Configures the max client request body size of nginx. _(Default: `0`, Disables checking of client request body size.)_
* `PORT`: This is the port that nginx will listen on _(Default: `80`)_

# Security

While most routers will perform routing only, we have added a very simple mechanism to do API Key based authorization
at the router level.  Why might you want this?  Imagine you've got multi-tenancy in Kubernetes where each namespace is
specific to a single tenant.  To avoid a Pod in namespace `X` configuring itself to receive traffic from namespace `Y`,
this router allows you to create a specially named secret _(`routing` in this case)_ with a specially named data field
_(`api-key`)_ and the value stored in this secret will be used to secure traffic to all Pods in your namespace wired up
for routing.  To do this, nginx is configured to ensure that all requests routed to your Pod have the
`X-ROUTING-API-KEY` header provided with its value being the base64-encoded value of your secret.

Here is an example of how you might create this secret so that all Pods wired up for routing in the `my-namespace`
namespace are secured via API Key:

```
kubectl create secret generic routing --from-literal=api-key=supersecret --namespace=my-namespace
```

Based on the example, any routes that points to Pods in the `my-namespace` namespace will be required to have
`X-ROUTING-API-KEY: c3VwZXJzZWNyZXQ=` set in their request for the router to allow routing to the Pods.  Otherwise, a
`403` is returned.  Of course, if your namespace does not have the specially named secret, you do not have to adhere to
provide this header.

**Note:** This feature is written assuming that each combination of `github.com/30x.dispatcher.hosts` and `github.com/30x.dispatcher.paths` will only be
configured such that the Pods servicing the traffic are from a single namespace.  Once you start allowing pods from
multiple namespaces to consume traffic for the same host and path combination, this falls apart.  While the routing will
work fine in this situation, the router's API Key is namespace specific and the first seen API Key is the one that is
used.

# Streaming Support

By default, nginx will buffer responses for proxied servers.  Unfortunately, this can be a problem if you deploy a
streaming APIs.  Thankfully, nginx makes it easy for proxied applications to disable proxy buffering by setting the
`X-Accel-Buffering` header to `no`.  Doing this will make your streaming API work as expected.  For more details, view
the nginx documentation: http://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffering

# WebSocket Support

Why are we bringing up WebSocket support?  Well, nginx itself operates in a way that makes routing to Pods that are
WebSocket servers a little difficult.  For more details, read the nginx documentation located here:
https://www.nginx.com/blog/websocket-nginx/  The way that the k8s-router addresses things is at each `location`
block, we throw in some WebSocket configuration.  It's very simple stuff but since there is some reasoning behind the
location where this is applied and the approach taken, it makes sense to explain it here.

The WebSocket configuration is at the `location` level, and it is there because nginx does not allow us to use the
[set directive](http://nginx.org/en/docs/http/ngx_http_rewrite_module.html#set) at the `server` or `http` level.  We
have to use the `set` directive to properly handle the `Connection` header.  See, nginx uses `close` as the default
value for the `Connection` header when there is no `Connection` header provided.  So if we just passed through the
`Connection` header and there was no `Connection` header provided, instead of using the default value of `close` the
value would be `''` which would basically delete the `Connection` header which is not how nginx operates.  So we have to
conditionally set a variable based on the `Connection` header value and `set` is the only way.

The other part of this implementation that is worth documenting is that in the previously linked documentation for
enabling WebSockets in nginx, you see they use `proxy_http_version 1.1;` to force HTTP 1.1.  Well, for a generic server
where not all `location` blocks are for WebSockets, we needed a way to conditionally enable HTTP 1.1.  Well...there is
no way to do this.  `proxy_http_version` cannot be used in an `if` directive and `proxy_http_version` cannot be set to
a string value, which is the only value you can use for nginx variables.  So since we do not want to force HTTP 1.1 on
everyone, we just leave it up to the client to make an HTTP 1.1 request.

So when you look at the generated nginx configuration and see some duplicate configuration related to WebSockets, or
you see that we are not forcing HTTP 1.1, now you know.

# Examples

## An Ingress Controller

Let's assume you've already deployed the router controller.  _(If you haven't, feel free to look at the
[Building and Running](#building-and-running) section of the documentation.)_  When the router starts up, nginx is
configured and started on your behalf.  The generated `/etc/nginx/nginx.conf` that the router starts with looks similar to
this _(assuming you do not have any deployed Pods marked for routing)_:

``` nginx
# A very simple nginx configuration file that forces nginx to start as a daemon.
events {}
http {
  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
```

This configuration will tell nginx to listen on port `80` and all requests for unknown hosts will be closed, as if there
was no server listening for traffic.  _(This approach is better than reporting a `404` because a `404` says someone is
there but the request was for a missing resource while closing the connection says that the request was for a server
that didn't exist, or in our case a request was made to a host that our router is unaware of.)_

Now that we know how the router spins up nginx initially, let's deploy a _microservice_ to Kubernetes.  To do that, we
will be packaging up a simple Node.js application that prints out the environment details of its running container,
including the IP address(es) of its host.  To do this, we will build a Docker image, publish the Docker image and then
create a Kubernetes ReplicationController that will deploy one pod representing our microservice.

**Note:** All commands you see in this demo assume you are already within the `demo` directory.
These commands are written assuming you are running docker at `192.168.64.1:5000` so please adjust your Docker commands
accordingly.

First things first, let's build our Docker image using `docker build -t nodejs-k8s-env .`, tag the Docker image using
`docker tag -f nodejs-k8s-env 192.168.64.1:5000/nodejs-k8s-env` and finally push the Docker image to your Docker
registry using `docker push 192.168.64.1:5000/nodejs-k8s-env`.  At this point, we have built and published a Docker
image for our microservice.

The next step is to deploy our microservice to Kubernetes but before we do this, let's look at the ReplicationController
configuration file to see what is going on.  Here is the `rc.yaml` we'll be using to deploy our microservice
_(Same as `demo/rc.yaml`)_:

``` yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: nodejs-k8s-env
  labels:
    name: nodejs-k8s-env
spec:
  replicas: 1
  selector:
    name: nodejs-k8s-env
  template:
    metadata:
      labels:
        name: nodejs-k8s-env
        # This marks the pod as routable
        github.com/30x.dispatcher.routable: "true"
        github.com/30x.dispatcher.app.name: "nodejs-k8s-env"
        github.com/30x.dispatcher.app.rev: "1"
      annotations:
        # This says that only traffic for the "/nodejs" path and its sub paths will be routed to this pod, on port 3000
        github.com/30x.dispatcher.paths: '[{"basePath": "/nodejs", "containerPort": "3000"}]'
    spec:
      containers:
      - name: nodejs-k8s-env
        image: 192.168.64.1:5000/nodejs-k8s-env
        env:
          - name: PORT
            value: "3000"
        ports:
          - containerPort: 3000
```

When we deploy our microservice using `kubectl create -f rc.yaml`, the router will notice that we now have one Pod
running that is marked for routing.  If you were tailing the logs, or you were to review the content of
`/etc/nginx/nginx.conf` in the container, you should see that it now reflects that we have a new microservice
deployed:

``` nginx
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Force HTTP 1.1 for upstream requests
  proxy_http_version 1.1;

  # When nginx proxies to an upstream, the default value used for 'Connection' is 'close'.  We use this variable to do
  # the same thing so that whenever a 'Connection' header is in the request, the variable reflects the provided value
  # otherwise, it defaults to 'close'.  This is opposed to just using "proxy_set_header Connection $http_connection"
  # which would remove the 'Connection' header from the upstream request whenever the request does not contain a
  # 'Connection' header, which is a deviation from the nginx norm.
  map $http_connection $p_connection {
    default $http_connection;
    ''      close;
  }

  # Pass through the appropriate headers
  proxy_set_header Connection $p_connection;
  proxy_set_header Host $host;
  proxy_set_header Upgrade $http_upgrade;

  # Upstream for /nodejs traffic on test.k8s.local
  upstream upstream1866206336 {
    # Pod nodejs-k8s-env-eq7mh
    server 10.244.69.6:3000;
  }

  server {
    listen 80;
    server_name test.k8s.local;

    location /nodejs {
      # Upstream upstream1866206336
      proxy_pass http://upstream1866206336;
    }
  }

  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
```

This means that if someone requests `http://test.k8s.local/nodejs`, assuming you've got `test.k8s.local` pointed to the
edge of your Kubernetes cluster, it should get routed to the proper Pod _(`nodejs-k8s-env-eq7mh` in our example)_.  If
everything worked out properly, you should see output like this:

``` json
{
  "env": {
    "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "HOSTNAME": "nodejs-k8s-env-eq7mh",
    "PORT": "3000",
    "KUBERNETES_PORT": "tcp://10.100.0.1:443",
    "KUBERNETES_PORT_443_TCP": "tcp://10.100.0.1:443",
    "KUBERNETES_PORT_443_TCP_PROTO": "tcp",
    "KUBERNETES_PORT_443_TCP_PORT": "443",
    "KUBERNETES_PORT_443_TCP_ADDR": "10.100.0.1",
    "KUBERNETES_SERVICE_HOST": "10.100.0.1",
    "KUBERNETES_SERVICE_PORT": "443",
    "VERSION": "v5.8.0",
    "NPM_VERSION": "3",
    "HOME": "/root"
  },
  "ips": {
    "lo": "127.0.0.1",
    "eth0": "10.244.69.6"
  }
}
```

If you've noticed in the example nginx configuration files above, nginx is configured appropriately to reverse proxy to
WebSocket servers.  _(The only reason we bring this up is that it's not something you get out of the box.)_  To test
this using the the previously-deployed application, use the following Node.js application:

```js
var socket = require('socket.io-client')('http://test.k8s.local', {path: '/nodejs/socket.io'});

socket.on('env', function (env) {
  console.log(JSON.stringify(env, null, 2));
});

// Emit the 'env' event to the server, which emits an 'env' event to the client with the server environment details.
socket.emit('env');
```

Now that's cool and all but what happens when we scale our application?  Let's scale our microservice to `3` instances
using `kubectl scale --replicas=3 replicationcontrollers nodejs-k8s-env`.  Your `/etc/nginx/nginx.conf` should look
something like this:

``` nginx
events {
  worker_connections 1024;
}
http {
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Force HTTP 1.1 for upstream requests
  proxy_http_version 1.1;

  # When nginx proxies to an upstream, the default value used for 'Connection' is 'close'.  We use this variable to do
  # the same thing so that whenever a 'Connection' header is in the request, the variable reflects the provided value
  # otherwise, it defaults to 'close'.  This is opposed to just using "proxy_set_header Connection $http_connection"
  # which would remove the 'Connection' header from the upstream request whenever the request does not contain a
  # 'Connection' header, which is a deviation from the nginx norm.
  map $http_connection $p_connection {
    default $http_connection;
    ''      close;
  }

  # Pass through the appropriate headers
  proxy_set_header Connection $p_connection;
  proxy_set_header Host $host;
  proxy_set_header Upgrade $http_upgrade;

  # Upstream for /nodejs traffic on test.k8s.local
  upstream upstream1866206336 {
    # Pod nodejs-k8s-env-eq7mh
    server 10.244.69.6:3000;
    # Pod nodejs-k8s-env-yr1my
    server 10.244.69.8:3000;
    # Pod nodejs-k8s-env-oq9xn
    server 10.244.69.9:3000;
  }

  server {
    listen 80;
    server_name test.k8s.local;

    location /nodejs {
      # Upstream upstream1866206336
      proxy_pass http://upstream1866206336;
    }
  }

  # Default server that will just close the connection as if there was no server available
  server {
    listen 80 default_server;
    return 444;
  }
}
```

In this example the only thing that changes is the number of Pods in the nginx [upstream](http://nginx.org/en/docs/http/ngx_http_upstream_module.html). 
Nginx will load balance across all pods using round-robin.

I hope this example gave you a better idea of how this all works.  If not, let us know how to make it better.

## Multipurpose Deployments

As mentioned above, this project started out as an ingress with the sole purpose of routing traffic from the internet
to Pods within the Kubernetes cluster.  One of the use cases we have at work is we need an general ingress but we also
want to use this router for a simplistic service router.  So essentially, we have a public ingress and a
private...router.  Here is an example deployment file where you use the configurability of this router to serve both
purposes:

``` yaml
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: k8s-pods-router
  labels:
    app: k8s-pods-router
spec:
  template:
    metadata:
      labels:
        app: k8s-pods-router
    spec:
      containers:
      - image: thirtyx/dispatcher:latest
        imagePullPolicy: Always
        name: k8s-pods-router-public
        ports:
          - containerPort: 80
            hostPort: 80
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          # Use the configuration to use the public/private paradigm (public version)
          - name: API_KEY_SECRET_FIELD
            value: public-api-key
          - name: HOSTS_ANNOTATION
            value: github.com/30x.dispatcher.hosts.public
          - name: PATHS_ANNOTATION
            value: github.com/30x.dispatcher.paths.public
      - image: thirtyx/dispatcher:latest
        imagePullPolicy: Always
        name: k8s-pods-router-private
        ports:
          - containerPort: 81
            # We should probably avoid using host port and if needed, at least lock it down from external access
            hostPort: 81
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          # Use the configuration to use the public/private paradigm (private version)
          - name: API_KEY_SECRET_FIELD
            value: private-api-key
          - name: HOSTS_ANNOTATION
            value: github.com/30x.dispatcher.hosts.private
          - name: PATHS_ANNOTATION
            value: github.com/30x.dispatcher.paths.private
          # Since we cannot have two containers listening on the same port, use a different port for the private router
          - name: PORT
            value: "81"
```

Based on this deployment, we have an ingress that serves `github.com/30x.dispatcher.hosts.public` and `github.com/30x.dispatcher.paths.public` combinations and an internal
router that serves `github.com/30x.dispatcher.hosts.private` and `github.com/30x.dispatcher.paths.private` combinations.  With this being the case, let's take our example
Node.js application deployed above and lets deploy a variant over it that has both public and private paths:

``` yaml
apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: null
  name: test-namespace
  annotations:
    github.com/30x.dispatcher.hosts.public: '{"test.k8s.com":{}}'
    github.com/30x.dispatcher.hosts.private: '{"test.k8s.local":{}}'
  labels:
    github.com/30x.dispatcher.routable: "true"
    github.com/30x.dispatcher.org: "some-org"
    github.com/30x.dispatcher.env: "test"

---

apiVersion: v1
kind: ReplicationController
metadata:
  name: nodejs-k8s-env
  labels:
    name: nodejs-k8s-env
spec:
  replicas: 1
  selector:
    name: nodejs-k8s-env
  template:
    metadata:
      labels:
        name: nodejs-k8s-env
        github.com/30x.dispatcher.routable: "true"
      annotations:
        github.com/30x.dispatcher.paths.private: '[{"basePath": "/internal", "containerPort": "3000"}]'
        github.com/30x.dispatcher.paths.public: '[{"basePath": "/public", "containerPort": "3000"}]'
    spec:
      containers:
      - name: nodejs-k8s-env
        image: thirtyx/nodejs-k8s-env
        env:
          - name: PORT
            value: "3000"
        ports:
          - containerPort: 3000
```

Now if we were to `curl http://test.k8s.com/nodejs` from outside of Kubernetes, assuming DNS was setup properly, the
ingress router would route properly but if we were to `curl http://test.k8s.local/nodejs`, it wouldn't go anywhere.  Not
only that, if I were to `curl http://test.k8s.com/internal`, it also would not go anywhere.  The only way to access the
`/internal` path would be to be within Kubernetes, with DNS properly setup, and to
`curl http://test.k8s.local/internal`.

Now I realize this is a somewhat convoluted example but the purpose was to show how we could use the same code base to
serve different roles using configuration alone.  Thet network isolation and security required to do this properly is
outside the scope of this example.

# Building and Running

If you want to run `dispatcher` in _mock mode_, you can use `go build` followed by `./dispatcher`.  Running in mock mode
means that `nginx` is not actually started and managed, nor is any actual routing performed.  `dispatcher` will use your
`kubectl` configuration to identify the cluster connection details by using the current context `kubectl` is configured
for.  This is very useful in the event you want to connect to an external Kubernetes cluster and see the generated
routing configuration.

If you're building this to run on Kubernetes, you'll need to do the following:

* `CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o dispatcher .`
* `docker build ...`
* `docker tag ...`
* `docker push ...`

_(The `...` are there because your Docker comands will likely be different than mine or someone else's)_  We have an
example DaemonSet for deploying the k8s-router as an ingress controller to Kubernetes located at
`examples/ingress-daemonset.yaml`.  Here is how I test locally:

* `CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o dispatcher .`
* `docker build -t dispatcher .`
* `docker tag -f dispatcher 192.168.64.1:5000/dispatcher`
* `docker push 192.168.64.1:5000/dispatcher`
* `kubectl create -f examples/ingress-daemonset.yaml`

**Note:** This router is written to be ran within Kubernetes but for testing purposes, it can be ran outside of
Kubernetes.  When ran outside of Kubernetes, you will have have a kube config file. When ran outside the container, nginx itself will not be
started and its configuration file will not be written to disk, only printed to stdout.  This might change in the future
but for now, this support is only as a convenience.

# Credit

This project is largly a refactor of [k8s-router](https://github.com/30x/k8s-router) with a few changes:
* Usage of namespace annotations for hosts (and their configuration)
* Usage of JSON documents in annotation values for hosts/paths configuration

