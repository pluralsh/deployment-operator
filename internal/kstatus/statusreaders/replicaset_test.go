package statusreaders_test

import (
	"context"
	"strings"
	"testing"

	"github.com/pluralsh/deployment-operator/internal/kstatus/statusreaders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fakecr "sigs.k8s.io/cli-utils/pkg/kstatus/polling/clusterreader/fake"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/testutil"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	fakemapper "sigs.k8s.io/cli-utils/pkg/testutil"
)

var (
	rsGVK = appsv1.SchemeGroupVersion.WithKind("ReplicaSet")
	rsGVR = appsv1.SchemeGroupVersion.WithResource("replicasets")
)

var (
	currentReplicaset = strings.TrimSpace(`
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  labels:
    app: guestbook
    plural.sh/managed-by: agent
    tier: frontend
  name: frontend
  namespace: test-do
  resourceVersion: "4869207"
  uid: 437e2329-59e4-42b9-ae40-48da3562d17e
spec:
  replicas: 3
  selector:
    matchLabels:
      tier: frontend
  template:
    metadata:
      creationTimestamp: null
      labels:
        tier: frontend
    spec:
      containers:
      - image: us-docker.pkg.dev/google-samples/containers/gke/gb-frontend:v5
        imagePullPolicy: IfNotPresent
        name: php-redis
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status:
  availableReplicas: 3
  fullyLabeledReplicas: 3
  observedGeneration: 1
  readyReplicas: 3
  replicas: 3
`)
)

func TestReplicaSetReadStatus(t *testing.T) {
	testCases := map[string]struct {
		identifier             object.ObjMetadata
		readerResource         *unstructured.Unstructured
		readerErr              error
		expectedErr            error
		expectedResourceStatus *event.ResourceStatus
	}{
		"Current resource": {
			identifier:     object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentReplicaset)),
			readerResource: testutil.YamlToUnstructured(t, currentReplicaset),
			expectedResourceStatus: &event.ResourceStatus{
				Identifier:         object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentReplicaset)),
				Status:             status.CurrentStatus,
				Resource:           testutil.YamlToUnstructured(t, currentReplicaset),
				Message:            "ReplicaSet is available. Replicas: 3",
				GeneratedResources: make(event.ResourceStatuses, 0),
			},
		},
		"Resource not found": {
			identifier: object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentReplicaset)),
			readerErr:  errors.NewNotFound(rsGVR.GroupResource(), "test"),
			expectedResourceStatus: &event.ResourceStatus{
				Identifier: object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentReplicaset)),
				Status:     status.NotFoundStatus,
				Message:    "Resource not found",
			},
		},
		"Context cancelled": {
			identifier:  object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentReplicaset)),
			readerErr:   context.Canceled,
			expectedErr: context.Canceled,
		},
	}

	for tn := range testCases {
		tc := testCases[tn]
		t.Run(tn, func(t *testing.T) {
			fakeReader := &fakecr.ClusterReader{
				GetResource: tc.readerResource,
				GetErr:      tc.readerErr,
			}
			fakeMapper := fakemapper.NewFakeRESTMapper(rsGVK)
			statusReader := statusreaders.NewReplicaSetStatusReader(fakeMapper)

			rs, err := statusReader.ReadStatus(context.Background(), fakeReader, tc.identifier)

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected error, but didn't get one")
				} else {
					assert.EqualError(t, err, tc.expectedErr.Error())
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResourceStatus, rs)
		})
	}
}
