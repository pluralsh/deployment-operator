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
	ssGVK = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
	ssGVR = appsv1.SchemeGroupVersion.WithResource("statefulsets")
)

var (
	currentStatefulset = strings.TrimSpace(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
  namespace: test-do
spec:
  minReadySeconds: 10
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Retain
    whenScaled: Retain
  podManagementPolicy: OrderedReady
  replicas: 3
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: nginx
  serviceName: nginx
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: nginx
    spec:
      containers:
      - image: registry.k8s.io/nginx-slim:0.24
        imagePullPolicy: IfNotPresent
        name: nginx
        ports:
        - containerPort: 80
          name: web
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /usr/share/nginx/html
          name: www
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 10
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      creationTimestamp: null
      name: www
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
      storageClassName: my-storage-class
      volumeMode: Filesystem
    status:
      phase: Pending
status:
  availableReplicas: 0
  collisionCount: 0
  currentReplicas: 1
  currentRevision: web-7757fc6447
  observedGeneration: 1
  replicas: 1
  updateRevision: web-7757fc6447
  updatedReplicas: 1
`)
)

func TestStateFulSetReadStatus(t *testing.T) {
	testCases := map[string]struct {
		identifier             object.ObjMetadata
		readerResource         *unstructured.Unstructured
		readerErr              error
		expectedErr            error
		expectedResourceStatus *event.ResourceStatus
	}{
		"In progress resource": {
			identifier:     object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentStatefulset)),
			readerResource: testutil.YamlToUnstructured(t, currentStatefulset),
			expectedResourceStatus: &event.ResourceStatus{
				Identifier:         object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentStatefulset)),
				Status:             status.InProgressStatus,
				Resource:           testutil.YamlToUnstructured(t, currentStatefulset),
				Message:            "Replicas: 1/3",
				GeneratedResources: make(event.ResourceStatuses, 0),
			},
		},
		"Resource not found": {
			identifier: object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentStatefulset)),
			readerErr:  errors.NewNotFound(ssGVR.GroupResource(), "test"),
			expectedResourceStatus: &event.ResourceStatus{
				Identifier: object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentStatefulset)),
				Status:     status.NotFoundStatus,
				Message:    "Resource not found",
			},
		},
		"Context cancelled": {
			identifier:  object.UnstructuredToObjMetadata(testutil.YamlToUnstructured(t, currentStatefulset)),
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
			fakeMapper := fakemapper.NewFakeRESTMapper(ssGVK)
			statusReader := statusreaders.NewStatefulSetResourceReader(fakeMapper)

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
