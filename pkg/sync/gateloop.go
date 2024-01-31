package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	console "github.com/pluralsh/console-client-go"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/api/pipelines/v1alpha1"
	"github.com/pluralsh/deployment-operator/generated/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	defer engine.gateQueue.Done(item)
	gate, ok := item.(*console.PipelineGateFragment)
	if !ok {
		// handle if assertion fails (shouldn't happen)
		err := fmt.Errorf("unexpected type: %T", item)
		log.Error(err, "failed to process gate item, ignoring for now", "error", err)
		return err
	}

	log.Info("attempting to sync gate", "Name", gate.Name, "ID", gate.ID)

	log.Info("syncing gate", "name", gate.Name)
	gateJSON, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		log.Error(err, "failed to marshalindent gate")
	}
	fmt.Printf("gate json from API: \n %s\n", string(gateJSON))

	if gate.Type != console.GateTypeJob {
		log.Info(fmt.Sprintf("gate is of type %s, we only reconcile gates of type %s skipping", gate.Type, console.GateTypeJob), "Name", gate.Name, "ID", gate.ID)
	}

	gateCR, err := engine.client.ParsePipelineGateCR(gate)
	if err != nil {
		log.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
		return err
	}
	updateOrCreatePipelineGate(engine.genClientset, gateCR)

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
	log.Info("Updated pipeline gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
	return nil
}
