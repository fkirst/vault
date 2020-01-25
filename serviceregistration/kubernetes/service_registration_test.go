package kubernetes

import (
	"sync"
	"testing"

	"github.com/hashicorp/go-hclog"
	sr "github.com/hashicorp/vault/serviceregistration"
	"github.com/hashicorp/vault/serviceregistration/kubernetes/client"
)

var testVersion = "version 1"

func TestServiceRegistration(t *testing.T) {
	testState, closeFunc := client.TestServer(t)
	defer closeFunc()

	if testState.NumPatches() != 0 {
		t.Fatalf("expected 0 patches but have %d: %+v", testState.NumPatches(), testState)
	}
	shutdownCh := make(chan struct{})
	config := map[string]string{
		"namespace": client.TestNamespace,
		"pod_name":  client.TestPodname,
	}
	logger := hclog.NewNullLogger()
	state := sr.State{
		VaultVersion:         testVersion,
		IsInitialized:        true,
		IsSealed:             true,
		IsActive:             true,
		IsPerformanceStandby: true,
	}
	reg, err := NewServiceRegistration(config, logger, state, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Run(shutdownCh, &sync.WaitGroup{}); err != nil {
		t.Fatal(err)
	}

	// Test initial state.
	if testState.NumPatches() != 5 {
		t.Fatalf("expected 5 current labels but have %d: %+v", testState.NumPatches(), testState)
	}
	if testState.Get(pathToLabels+labelVaultVersion).Value != testVersion {
		t.Fatalf("expected %q but received %q", testVersion, testState.Get(pathToLabels+labelVaultVersion).Value)
	}
	if testState.Get(pathToLabels+labelActive).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelActive).Value)
	}
	if testState.Get(pathToLabels+labelSealed).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelSealed).Value)
	}
	if testState.Get(pathToLabels+labelPerfStandby).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelPerfStandby).Value)
	}
	if testState.Get(pathToLabels+labelInitialized).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelInitialized).Value)
	}

	// Test NotifyActiveStateChange.
	if err := reg.NotifyActiveStateChange(false); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelActive).Value != toString(false) {
		t.Fatalf("expected %q but received %q", toString(false), testState.Get(pathToLabels+labelActive).Value)
	}
	if err := reg.NotifyActiveStateChange(true); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelActive).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelActive).Value)
	}

	// Test NotifySealedStateChange.
	if err := reg.NotifySealedStateChange(false); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelSealed).Value != toString(false) {
		t.Fatalf("expected %q but received %q", toString(false), testState.Get(pathToLabels+labelSealed).Value)
	}
	if err := reg.NotifySealedStateChange(true); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelSealed).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelSealed).Value)
	}

	// Test NotifyPerformanceStandbyStateChange.
	if err := reg.NotifyPerformanceStandbyStateChange(false); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelPerfStandby).Value != toString(false) {
		t.Fatalf("expected %q but received %q", toString(false), testState.Get(pathToLabels+labelPerfStandby).Value)
	}
	if err := reg.NotifyPerformanceStandbyStateChange(true); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelPerfStandby).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelPerfStandby).Value)
	}

	// Test NotifyInitializedStateChange.
	if err := reg.NotifyInitializedStateChange(false); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelInitialized).Value != toString(false) {
		t.Fatalf("expected %q but received %q", toString(false), testState.Get(pathToLabels+labelInitialized).Value)
	}
	if err := reg.NotifyInitializedStateChange(true); err != nil {
		t.Fatal(err)
	}
	if testState.Get(pathToLabels+labelInitialized).Value != toString(true) {
		t.Fatalf("expected %q but received %q", toString(true), testState.Get(pathToLabels+labelInitialized).Value)
	}
}
