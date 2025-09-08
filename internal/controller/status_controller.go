package controller

// TODO: can we get rid of this now?
// const (
//	StatusFinalizer = "deployments.plural.sh/inventory-protection"
//)
//
// type StatusReconciler struct {
//	k8sClient.Client
//	inventoryCache cache.InventoryResourceKeys
//}
//
// func (r *StatusReconciler) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
//	logger := log.FromContext(ctx)
//
//	configMap := &corev1.ConfigMap{}
//	if err := r.Get(ctx, req.NamespacedName, configMap); err != nil {
//		logger.Info("unable to fetch configmap")
//		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
//	}
//
//	scope, err := NewDefaultScope(ctx, r.Client, configMap)
//	if err != nil {
//		logger.Error(err, "failed to create configmap definition scope")
//		return ctrl.Result{}, err
//	}
//
//	// Always patch object when exiting this function, so we can persist any object changes.
//	defer func() {
//		if err := scope.PatchObject(); err != nil && reterr == nil {
//			reterr = err
//		}
//	}()
//
//	if !configMap.DeletionTimestamp.IsZero() {
//		// delete finalizer for legacy objects
//		controllerutil.RemoveFinalizer(configMap, StatusFinalizer)
//		return ctrl.Result{}, nil
//	}
//
//	inv, err := common.ToUnstructured(configMap)
//	if err != nil {
//		return ctrl.Result{}, err
//	}
//
//	set, err := inventory.WrapInventoryObj(inv).Load()
//	if err != nil {
//		return ctrl.Result{}, err
//	}
//
//	invID := r.inventoryID(configMap)
//
//	// If services arg is provided, we can skip
//	// services that are not on the list.
//	if args.SkipService(invID) {
//		return ctrl.Result{}, nil
//	}
//
//	r.inventoryCache[invID] = cache.ResourceKeyFromObjMetadata(set)
//	cache.GetResourceCache().Register(r.inventoryCache.Values().TypeIdentifierSet())
//
//	return ctrl.Result{}, reterr
//}
//
//// SetupWithManager sets up the controller with the Manager.
// func (r *StatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
//	return ctrl.NewControllerManagedBy(mgr).
//		For(&corev1.ConfigMap{}).
//		WithEventFilter(predicate.NewPredicateFuncs(func(o k8sClient.Object) bool {
//			_, exists := o.GetLabels()[cliutilscommon.InventoryLabel]
//			return exists
//		})).
//		WithEventFilter(withInventoryEventFilter(r.inventoryCache)).
//		Complete(r)
//}
//
// func withInventoryEventFilter(inventoryCache cache.InventoryResourceKeys) predicate.Predicate {
//	return predicate.Funcs{
//		CreateFunc: func(e event.CreateEvent) bool {
//			return true
//		},
//		UpdateFunc: func(e event.UpdateEvent) bool {
//			return true
//		},
//		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
//			if deleteEvent.Object == nil {
//				return false
//			}
//			inventoryID, exists := deleteEvent.Object.GetLabels()[cliutilscommon.InventoryLabel]
//			if exists {
//				deleteFromInventoryCache(inventoryCache, inventoryID)
//			}
//			return true
//		},
//		GenericFunc: func(e event.GenericEvent) bool {
//			return false
//		},
//	}
//}
//
// func deleteFromInventoryCache(inventoryCache cache.InventoryResourceKeys, inventoryID string) {
//	delete(inventoryCache, inventoryID)
//	cache.GetResourceCache().Unregister(inventoryCache.Values().TypeIdentifierSet())
//}
//
// func (r *StatusReconciler) inventoryID(c *corev1.ConfigMap) string {
//	return c.Labels[cliutilscommon.InventoryLabel]
//}
//
// func NewStatusReconciler(c k8sClient.Client) *StatusReconciler {
//	return &StatusReconciler{
//		Client:         c,
//		inventoryCache: make(cache.InventoryResourceKeys),
//	}
//}
