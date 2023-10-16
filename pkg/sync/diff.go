package sync

/*func (engine *Engine) diff(manifests []*unstructured.Unstructured, namespace, svcId string) (*diff.DiffResultList, error) {
	liveObjs, err := engine.cache.GetManagedLiveObjs(manifests, isManaged(svcId))
	if err != nil {
		return nil, err
	}

	reconciliation := sync.Reconcile(manifests, liveObjs, namespace, engine.cache)
	return diff.DiffArray(reconciliation.Target, reconciliation.Live, diff.WithManager(SSAManager), diff.WithLogr(log))
}*/
