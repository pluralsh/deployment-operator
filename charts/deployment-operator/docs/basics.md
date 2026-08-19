# Plural Deployment Operator

The deployment operator is both a gitops sync agent and a kubernetes operator that applies any manifest or state change in Plural CD into your cluster.  It's meant to be lightweight but there are some configuration settings you can add as needed.  Here's an overview:

| Name | Description | Default |
| ---- | ----------- | ------- |
| tag | whether you want to use a different docker tag | <AppVersion> |
| replicas | number of replicas for the operator (set to 0 to disable) | 1 |
| cacheHostPathEnabled | persist operator caches on a node-local hostPath (`replicaCount` must stay 1) | false |
| cachePersistInterval | how often to flush in-memory caches to the cache dir | 10s |
| cacheHostPath | host path used when cache hostPath is enabled | /var/lib/plural/deployment-operator |
| cacheDir | mount path for the cache inside the operator container | /plural/cache |