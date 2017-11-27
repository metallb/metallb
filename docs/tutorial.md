# Tutorial

In this tutorial, we'll set up a BGP router in Minikube, configure
MetalLB to use it, and create some load-balanced services. We'll be
able to inspect the BGP router's state, and see that it reflects the
intent that we expressed in Kubernetes.

Because this will be a simulated environment inside Minikube, this
setup only lets you inspect the router's state and see what it _would_
do in a real deployment. Once you've experimented in this setting and
are ready to set up MetalLB on a real cluster, refer to
the [installation guide]() for instructions.

Here is the outline of what we're going to do:
1. Start a Minikube cluster,
2. Set up a mock BGP router that we can inspect in subsequent steps,
3. Install MetalLB on the cluster,
4. Configure MetalLB to peer with our mock BGP router, and give it some IP addresses to manage,
5. Create a load-balanced service, and observe how MetalLB sets it up,
6. Tweak MetalLB's configuration, to see how the cluster reacts.

