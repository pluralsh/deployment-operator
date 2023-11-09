package resource

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHasAnnotationOption(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
		key string
		val string
	}
	tests := []struct {
		name     string
		args     args
		wantVals []string
		want     bool
	}{
		{"Nil", args{NewPod(), "foo", "bar"}, nil, false},
		{"Empty", args{example(""), "foo", "bar"}, nil, false},
		{"Single", args{example("bar"), "foo", "bar"}, []string{"bar"}, true},
		{"DeDup", args{example("bar,bar"), "foo", "bar"}, []string{"bar"}, true},
		{"Double", args{example("bar,baz"), "foo", "baz"}, []string{"bar", "baz"}, true},
		{"Spaces", args{example("bar "), "foo", "bar"}, []string{"bar"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.wantVals, GetAnnotationCSVs(tt.args.obj, tt.args.key))
			assert.Equal(t, tt.want, HasAnnotationOption(tt.args.obj, tt.args.key, tt.args.val))
		})
	}
}

func example(val string) *unstructured.Unstructured {
	return Annotate(NewPod(), "foo", val)
}

var PodManifest = `
{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "my-pod"
  },
  "spec": {
    "containers": [
      {
        "image": "nginx:1.7.9",
        "name": "nginx",
        "resources": {
          "requests": {
            "cpu": 0.2
          }
        }
      }
    ]
  }
}
`

func NewPod() *unstructured.Unstructured {
	return Unstructured(PodManifest)
}

func Annotate(obj *unstructured.Unstructured, key, val string) *unstructured.Unstructured {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = val
	obj.SetAnnotations(annotations)
	return obj
}

func Unstructured(text string) *unstructured.Unstructured {
	un := &unstructured.Unstructured{}
	var err error
	if strings.HasPrefix(text, "{") {
		err = json.Unmarshal([]byte(text), &un)
	} else {
		err = yaml.Unmarshal([]byte(text), &un)
	}
	if err != nil {
		panic(err)
	}
	return un
}
