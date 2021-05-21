<p align="center">
    <img src="./logo.svg" width="200">
</p>

## **This is a Prototype (Not suitable for production)**
# P|rometheus E|xporter for M|ongoDB A|tlas
Prometheus metrics exporter for MongoDB Atlas. This acts as a supplement to the official MongoDB Atlas - Datadog Integration. pema has no dependency on Datadog whatsoever. It exposes metrics in Prometheus's format. 

## Supported metrics at the moment
### 1. Active clusters (`pema_mongodb_atlas_active_clusters`)

Exposes active clusters

This metric was primarily created to facilitate creating alerts based on number of active clusters. e.g., If you run a backup/restore job to replace old clusters with the new clusters containing fresh data from production for development, there's a chance your job might fail and you'd have old clusters running which nobody is using. This incurs cost. 

## What's with the name and the logo?
Pema means lotus in [Tibetan](https://en.wikipedia.org/wiki/Pema). 

The logo is a svg vector in public domain. [Source ](https://openclipart.org/detail/171674/lotus-blossom)
