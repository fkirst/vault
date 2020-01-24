package kubernetes

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/serviceregistration/kubernetes/client"
)

type retryHandler struct {
	logger             hclog.Logger
	namespace, podName string
	client             *client.Client

	// patchesToRetry can only hold up to 4 patches because there are only 4
	// notifications, maximum, that can be out of date at once.
	patchesToRetry []*client.Patch

	// We need a mutex for this because the code mutates it in multiple
	// goroutines.
	patchesToRetryLock sync.Mutex
}

// TODO tests
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

// TODO tests
func (r *retryHandler) Add(patch *client.Patch) error {
	r.patchesToRetryLock.Lock()
	defer r.patchesToRetryLock.Unlock()

	// - If the patch is a dupe, don't add it.
	// - If the patch reverts another, remove them both.
	//     For example, perhaps we were already retrying "active = true",
	//     but this new patch tells us "active = false" again.
	// - Otherwise, this is a new, unique patch, so add this patch to retries.
	for i := 0; i < len(r.patchesToRetry); i++ {
		patchToRetry := r.patchesToRetry[i]
		if patch.Operation != patchToRetry.Operation {
			continue
		}
		if patch.Path != patchToRetry.Path {
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
		patchToRetryVal, _ := strconv.ParseBool(patchToRetry.Value.(string))
		if patchVal == patchToRetryVal {
			// We don't need to do anything because it already exists.
			return nil
		} else {
			// We need to delete its opposite from the slice.
			r.patchesToRetry = append(r.patchesToRetry[:i], r.patchesToRetry[i+1:]...)
		}
	}
	r.patchesToRetry[len(r.patchesToRetry)+1] = patch
	return nil
}

// TODO tests - and break this off into being a separate handler again, it's much easier to understand that way
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
	r.patchesToRetry = make([]*client.Patch, 0, 4)
}
