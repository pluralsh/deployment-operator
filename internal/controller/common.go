package controller

import (
	"github.com/samber/lo"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

func ensureBindings(bindings []v1alpha1.Binding, userGroupCache cache.UserGroupCache) ([]v1alpha1.Binding, bool, error) {
	requeue := false
	for i := range bindings {
		binding, req, err := ensureBinding(bindings[i], userGroupCache)
		if err != nil {
			return bindings, req, err
		}

		requeue = requeue || req
		bindings[i] = binding
	}

	return bindings, requeue, nil
}

func ensureBinding(binding v1alpha1.Binding, userGroupCache cache.UserGroupCache) (v1alpha1.Binding, bool, error) {
	requeue := false
	if binding.GroupName == nil && binding.UserEmail == nil {
		return binding, requeue, nil
	}

	if binding.GroupName != nil {
		groupID, err := userGroupCache.GetGroupID(*binding.GroupName)
		if err != nil && !errors.IsNotFound(err) {
			return binding, requeue, err
		}

		requeue = errors.IsNotFound(err)
		binding.GroupID = lo.EmptyableToPtr(groupID)
	}

	if binding.UserEmail != nil {
		userID, err := userGroupCache.GetUserID(*binding.UserEmail)
		if err != nil && !errors.IsNotFound(err) {
			return binding, requeue, err
		}

		requeue = errors.IsNotFound(err)
		binding.UserID = lo.EmptyableToPtr(userID)
	}

	return binding, requeue, nil
}
