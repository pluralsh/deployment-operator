package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	console "github.com/pluralsh/console-client-go"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/apis/pipelines/v1alpha1"
	"github.com/pluralsh/deployment-operator/generated/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//const (
//	// The field manager name for the ones agentk owns, see
//	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
//	fieldManager = "application/apply-patch"
//)

func (engine *Engine) GateControlLoop() {
	if engine.deathChan != nil {
		defer func() {
			if r := recover(); r != nil {
				engine.deathChan <- r
				fmt.Printf("panic: %s\n", string(debug.Stack()))
			}
		}()
	}

	for i := 0; i < workerCount; i++ {
		go engine.gateWorkerLoop()
	}
}

func (engine *Engine) gateWorkerLoop() {
	log.Info("starting sync worker for gates")
	for {
		log.Info("polling for new gate updates")
		gate, shutdown := engine.gateQueue.Get()
		if shutdown {
			log.Info("shutting down worker")
			break
		}
		err := engine.processGate(gate)
		if err != nil {
			log.Error(err, "failed to process gate")
		}
		time.Sleep(syncDelay)
	}
}

func (engine *Engine) processGate(item interface{}) error {
	//state truth is still always in the console!
	//so logic should most likely be
	//1. get the PipelineGate
	//2. if status is OPEN, then sync the gate on the cluster and set status to PENDING
	//  - if the gate is already synced into the cluster, i.e. CRD created, then do nothing, this can be the case if the gate was already synced,
	//	but reconciliation wasn't quick enough failed to update the status
	//  - only way to check if the gate is synced is to check if the CRD object exists -> k8s API call
	//3. if status is CLOSED, then do nothing, reconciliation will take care of clean up
	//4. if status is PENDING, it has already been synced into the cluster, so do nothing
	defer engine.gateQueue.Done(item)
	gate, ok := item.(*console.PipelineGateFragment)
	if !ok {
		// handle if assertion fails (shouldn't happen)
		err := fmt.Errorf("unexpected type: %T", item)
		log.Error(err, "failed to process gate item, ignoring for now", "error", err)
		return err
	}

	log.Info("attempting to sync gate", "Name", gate.Name, "ID", gate.ID)
	// TODO: shouldn't it always be in the gate cache? and if it's not do we put it there?
	//gate, err := engine.gateCache.Get(gate.ID, gate)
	//if err != nil {
	//	fmt.Printf("failed to fetch gate: %s, ignoring for now", err)
	//	return err
	//}

	log.Info("syncing gate", "name", gate.Name)
	gateJSON, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		log.Error(err, "failed to marshalindent gate")
	}
	fmt.Printf("gate json from API: \n %s\n", string(gateJSON))

	if gate.Type != console.GateTypeJob {
		log.Info(fmt.Sprintf("gate is of type %s, we only reconcile gates of type %s skipping", gate.Type, console.GateTypeJob), "Name", gate.Name, "ID", gate.ID)
	}

	//if gate.State == console.GateStateOpen {
	//	log.Info(fmt.Sprintf("gate is %s, skipping", gate.State), "Name", gate.Name, "ID", gate.ID)
	//	return nil
	//}
	gateCR, err := engine.client.ParsePipelineGateCR(gate)
	if err != nil {
		log.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
		return err
	}
	updateOrCreatePipelineGate(engine.genClientset, gateCR)

	//if gate.State == console.GateStatePending || gate.State == console.GateStateClosed {
	//	gateCR, err := engine.client.ParsePipelineGateCR(gate)
	//	if err != nil {
	//		log.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
	//		return err
	//	}
	//	gateCRJSON, err := json.MarshalIndent(gateCR, "", "  ")
	//	if err != nil {
	//		log.Error(err, "failed to marshalindent gateCR")
	//	}
	//	fmt.Printf("parsed gateCR json:\n %s\n", string(gateCRJSON))
	//	updateOrCreatePipelineGate(engine.genClientset, gateCR, gate)
	//	return nil
	//}

	//if gate.State == console.GateStateClosed {
	//	// parse a gate CR from the gate fragment
	//	gateCR, err := engine.client.ParsePipelineGateCR(gate)
	//	if err != nil {
	//		log.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
	//		return err
	//	}
	//	updateOrCreatePipelineGate(engine.genClientset, gateCR, gate)
	//	log.Info("gate synced", "Name", gate.Name, "ID", gate.ID)
	//	//gateState := console.GateStatePending
	//	//// TODO: add job ref, if it exists, but at this point it most likely doesn't because it hasn't been reconcilced yet
	//	//log.Info("update gate state in console", "Name", gate.Name, "ID", gate.ID)
	//	//// TODO: actually get the job ref, but it could be that the gate has no job ref
	//	//engine.client.UpdateGate(gate.ID, console.GateUpdateAttributes{State: &gateState, Status: &console.GateStatusAttributes{JobRef: &console.NamespacedName{Name: gateCreated.Name, Namespace: gateCreated.Namespace}}})
	//}
	return nil
}

func updateOrCreatePipelineGate(clientset *versioned.Clientset, gateCR *pipelinesv1alpha1.PipelineGate) error {
	gateCRJSON, err := json.MarshalIndent(gateCR, "", "  ")
	if err != nil {
		log.Error(err, "failed to marshalindent gateCR")
	}
	fmt.Printf("updating or creating gateCR json:\n %s\n", string(gateCRJSON))
	pgClient := clientset.PipelinesV1alpha1().PipelineGates(gateCR.Namespace)
	_, err = pgClient.Update(context.Background(), gateCR, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the PipelineGate doesn't exist, create it.
			_, err = pgClient.Create(context.Background(), gateCR, metav1.CreateOptions{})
			if err != nil {
				log.Error(err, "failed to create gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
				return err
			}
		} else {

			log.Error(err, "failed to update gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
			return err
		}
	}
	log.Info("Updated pipeline gate", "Namespace", gateCR.Namespace, "Name", gate.Name, "ID", gate.ID)
	return nil
}
