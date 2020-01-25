package kubernetes

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/serviceregistration/kubernetes/client"
)

// How often to retry sending a state update if it fails.
var retryFreq = 5 * time.Second

// retryHandler executes retries.
// It is thread-safe.
type retryHandler struct {
	// These don't need a mutex because they're never mutated.
	logger             hclog.Logger
	namespace, podName string
	client             *client.Client

	// This gets mutated on multiple threads.
	patchesToRetry     []*client.Patch
	patchesToRetryLock sync.Mutex
}

// Run runs at an interval, checking if anything has failed and if so,
// attempting to send them again.
func (r *retryHandler) Run(shutdownCh <-chan struct{}, wait *sync.WaitGroup) {
	// Make sure Vault will give us time to finish up here.
	wait.Add(1)
	defer wait.Done()

	retry := time.NewTicker(retryFreq)
	for {
		select {
		case <-shutdownCh:
			return
		case <-retry.C:
			r.retry()
		}
	}
}

// Add adds a patch to be retried until it's either completed without
// error, or no longer needed.
func (r *retryHandler) Add(patch *client.Patch) error {
	r.patchesToRetryLock.Lock()
	defer r.patchesToRetryLock.Unlock()

	// - If the patch is a dupe, don't add it.
	// - If the patch reverts another, remove them both.
	//     For example, perhaps we were already retrying "active = true",
	//     but this new patch tells us "active = false" again.
	// - Otherwise, this is a new, unique patch, so add this patch to retries.
	for i := 0; i < len(r.patchesToRetry); i++ {
		prevPatch := r.patchesToRetry[i]
		if patch.Path != prevPatch.Path {
			continue
		}
		if patch.Operation != prevPatch.Operation {
			continue
		}
		patchValStr, ok := patch.Value.(string)
		if !ok {
			return fmt.Errorf("all patches must have bool values but received %+x", patch)
		}
		patchVal, err := strconv.ParseBool(patchValStr)
		if err != nil {
			return err
		}
		// This was already verified to not be a bool string
		// when it was added to the slice.
		prevPatchVal, _ := strconv.ParseBool(prevPatch.Value.(string))
		if patchVal == prevPatchVal {
			// We don't need to do anything because it already exists.
			return nil
		} else {
			// We need to delete its opposite from the slice.
			r.patchesToRetry = append(r.patchesToRetry[:i], r.patchesToRetry[i+1:]...)
			return nil
		}
	}
	r.patchesToRetry = append(r.patchesToRetry, patch)
	return nil
}

func (r *retryHandler) retry() {
	r.patchesToRetryLock.Lock()
	defer r.patchesToRetryLock.Unlock()

	if len(r.patchesToRetry) == 0 {
		// Nothing to do here.
		return
	}

	if err := r.client.PatchPod(r.namespace, r.podName, r.patchesToRetry...); err != nil {
		if r.logger.IsWarn() {
			r.logger.Warn("unable to update state due to %s, will retry", err.Error())
		}
		return
	}
	r.patchesToRetry = make([]*client.Patch, 0)
}
