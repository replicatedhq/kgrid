#!/bin/bash

if [[ -z "$AWS_ACCESS_KEY_ID" ]]; then
    echo "AWS_ACCESS_KEY_ID must be set" 1>&2
    exit 1
fi

if [[ -z "$AWS_SECRET_ACCESS_KEY" ]]; then
    echo "AWS_SECRET_ACCESS_KEY must be set" 1>&2
    exit 1
fi

kubectl apply -f - << EOF
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
         value: ${AWS_ACCESS_KEY_ID}
       secretAccessKey:
         value: ${AWS_SECRET_ACCESS_KEY}
EOF

kubectl apply -f - << EOF
apiVersion: kgrid.replicated.com/v1alpha1
kind: Version
metadata:
  name: version
  namespace: kgrid-system
spec:
  kots:
    latest: v1.50.2
EOF
