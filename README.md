<p align="center">
    <img src="./logo.svg" width="200">
</p>

## **This is a Prototype (Not suitable for production)**
# P|rometheus E|xporter for M|ongoDB A|tlas
Prometheus metrics exporter for MongoDB Atlas. This acts as a supplement to the official MongoDB Atlas - Datadog Integration. pema has no dependency on Datadog whatsoever. It exposes metrics in Prometheus's format. 

## Getting started
### Installation
If you have kubectl 1.14 or above, you can just do 
```
$ kubectl apply -f k8s/
```
If you are running something older than 1.14, install [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) and run
```
$ kustomize build k8s/ | kubectl -f -
```
### Settings
Let's take a look at a sample settings file (k8s/settings)
```yaml
# (required) This field is used to fetch cluster information. Pema works on a MongoDB Atlas project.
projectId: "<MongoDB Atlas Project ID here>"
# (required) tags/labels you want to add
tags:
    # tag name. Has to be a valid yaml
    mongodb_version:
      # fetch MongoDBVersion field from 
      # every Cluster. Cluster here is a Golang Struct
      # defined [here](https://pkg.go.dev/go.mongodb.org/atlas/mongodbatlas#Cluster). Just imagine it as a key-value map
      # if you are not famililar with programming
      value: Cluster.MongoDBVersion
    id:
      # fetch ID from every cluster
      value: Cluster.ID
    name:
      # fetch Name from every cluster
      value: Cluster.Name
    # You can add as many tags as you want
    aya_type:
      value:
        # set values conditionally
        # Uses [Expr](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md) expressions
        # Here, it means get Name field from every cluster
        # and check if it `matches` the regex '.*-staging.*'
        # and if that is so, set the value to `staging`
        - if: Cluster.Name matches ".*-staging.*" # if-then A
          then: "staging"
        # if-then is evaluated in the order they are written in
        # i.e., if-then A is evaluated before if-then B  
        # Evluation method:
        # - if if-then A returns true, stop and use the `value`
        # field from if-then A
        # - if if-then A returns false, evaluate if-then B
        # -- if if-then B returns true, stop and use the value defined
        #    in the 'value' field of if-then B
        # -- if if-then B returns false, set the final value to empty string "".
        #    
        - if: Cluster.Name matches  ".*-develop" # if-then B
          then: "develop"
```
You can find a sample setting file in `k8s/settings`. Replace the content
with your config. 
### Things to note
- Make sure you don't change name of the `k8s/settings` file. Check the file to see why.
- Editing the settings in the ConfigMap won't automatically load the settings in the running application. You will have to restart the Pod.
- You need MongoDB Atlas keys to connect to the cluster. Check 'k8s/secret.yaml'. Don't rename the keys in this secret. Just set your Base64 encoded values against the  keys. 
## Supported metrics at the moment
### 1. Active clusters (`pema_mongodb_atlas_active_clusters`)

Exposes active clusters

This metric was primarily created to facilitate creating alerts based on number of active clusters. e.g., If you run a backup/restore job to replace old clusters with the new clusters containing fresh data from production for development, there's a chance your job might fail and you'd have old clusters running which nobody is using. This incurs cost. 

## What's with the name and the logo?
Pema means lotus in [Tibetan](https://en.wikipedia.org/wiki/Pema). 

The logo is a svg vector in public domain. [Source ](https://openclipart.org/detail/171674/lotus-blossom)
