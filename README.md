# kgrid

A Kubernetes operator to create test clusters and manage test runs on the clusters.

## Deploying

The operator can be deployed to a kubernetes by applying the YAML specs from release assets.

## Defining a test grid

Currenly only EKS clusters are supported for testing.  These can be defined by deploying a `Grid` spec.  For example:

```yaml
apiVersion: kgrid.replicated.com/v1alpha1
kind: Grid
metadata:
 name: test-kgrid
 namespace: kgrid-system
spec:
 clusters:
   - name: test-cluster
     eks:
       region: us-west-1
       version: "1.18"
       create: true
       accessKeyId:
         valueFrom:
           secretKeyRef:
             name: aws-access
             key: AWS_ACCESS_KEY_ID
       secretAccessKey:
         valueFrom:
           secretKeyRef:
             name: aws-access
             key: AWS_SECRET_ACCESS_KEY
```

## Defining an application

Currently only KOTS applications are supported.  An application can be defined by deploying an `Application` spec.  For example:

```yaml
apiVersion: kgrid.replicated.com/v1alpha1
kind: Application
metadata:
 name: my-app
 namespace: kgrid-system
spec:
 kots:
   clusters:
     - test-cluster
   version: "latest"
   appSlug: myappp
   licenseID: <license-id>
   channelID: <channel-id>
   channelSequence: <channel-sequence>
   skipPreflights: true
   namespace: test
   configValues:
     spec:
       values:
         a_flag:
           value: "0"
         a_text_value:
           value: test value
```

KOTS version can be specified in the `version` field.  However, if the value `latest` is used, the release version will be looked up in the `Version` object deployed to the same namespace.  For example:

```yaml
apiVersion: kgrid.replicated.com/v1alpha1
kind: Version
metadata:
 name: version
 namespace: kgrid-system
spec:
 kots:
   latest: "1.41.0"
```

## Running locally

1. Install CRDs: `make install`
1. Build images: `make docker-build docker-push docker-build-kgrid docker-push-kgrid`
1. Deploy controller: `make deploy`
1. Create `grid` (required) and `version` (optional) specs: `./dev/deploy_dev_specs.sh`

## RBAC

In order for a k8s ServiceAccount that does not have cluster level access to be able to manage kgrid application specs, it must have editor access.  The following RoleBinding needs to be created.

```
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: <name>
  namespace: kgrid-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kgrid-application-editor-role
subjects:
- kind: ServiceAccount
  name: <name>
  namespace: <name>
```