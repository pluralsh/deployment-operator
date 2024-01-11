package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LuaScriptScope struct {
	Client        client.Client
	HealthConvert *v1alpha1.HealthConvert
	ctx           context.Context
}

func (p *LuaScriptScope) PatchObject() error {

	key := client.ObjectKeyFromObject(p.HealthConvert)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oldScript := &v1alpha1.HealthConvert{}
		if err := p.Client.Get(p.ctx, key, oldScript); err != nil {
			return fmt.Errorf("could not fetch current %s/%s state, got error: %+v", oldScript.GetName(), oldScript.GetNamespace(), err)
		}

		if reflect.DeepEqual(oldScript.Status, p.HealthConvert.Status) {
			return nil
		}

		return p.Client.Status().Patch(p.ctx, p.HealthConvert, client.MergeFrom(oldScript))
	})

}

func NewClusterScope(ctx context.Context, client client.Client, luaScript *v1alpha1.HealthConvert) (*LuaScriptScope, error) {
	if luaScript == nil {
		return nil, errors.New("failed to create new cluster scope, got nil cluster")
	}
	return &LuaScriptScope{
		Client:        client,
		HealthConvert: luaScript,
		ctx:           ctx,
	}, nil
}
