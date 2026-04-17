// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"context"
	"fmt"

	snapshotclient "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	snapshotinformers "github.com/kubernetes-csi/external-snapshotter/client/v8/informers/externalversions"
	snapshotlisters "github.com/kubernetes-csi/external-snapshotter/client/v8/listers/volumesnapshot/v1"
	"github.com/rs/zerolog"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// InformerManager runs SharedInformerFactories for core PersistentVolumes and optional VolumeSnapshotContents.
type InformerManager struct {
	kubeClient              clientset.Interface
	snapshotClient          snapshotclient.Interface
	coreInformerFactory     informers.SharedInformerFactory
	snapshotInformerFactory snapshotinformers.SharedInformerFactory

	// PersistentVolume informer
	pvInformer cache.SharedInformer
	// Function to determine if pvInformer has been synced
	pvSynced cache.InformerSynced

	// VolumeSnapshotContent informer
	vscInformer cache.SharedInformer
	// Function to determine if vscInformer has been synced
	vscSynced cache.InformerSynced
}

// AddFunc is an informer OnAdd callback signature.
type AddFunc func(obj any)

// UpdateFunc is an informer OnUpdate callback signature.
type UpdateFunc func(oldObj any, newObj any)

// RemoveFunc is an informer OnDelete callback signature.
type RemoveFunc func(obj any)

// NewInformerManager constructs factories for kubeClient and, when snapshotClient is non-nil, VolumeSnapshotContents.
//
// kubeClient must be non-nil. snapshotClient may be nil if the snapshot API is not used.
// It returns a ready *InformerManager without starting informers yet.
func NewInformerManager(kubeClient clientset.Interface, snapshotClient snapshotclient.Interface) *InformerManager {
	im := &InformerManager{
		kubeClient:          kubeClient,
		snapshotClient:      snapshotClient,
		coreInformerFactory: informers.NewSharedInformerFactory(kubeClient, 0),
	}
	if snapshotClient != nil {
		im.snapshotInformerFactory = snapshotinformers.NewSharedInformerFactory(snapshotClient, 0)
	}
	return im
}

// AddPVListener registers resource event handlers on the PersistentVolume informer, creating it if needed.
//
// add, update, and remove are invoked from the informer thread; they must not block.
// It returns an error if the handler cannot be registered.
func (im *InformerManager) AddPVListener(add AddFunc, update UpdateFunc, remove RemoveFunc) error {
	if im.pvInformer == nil {
		im.pvInformer = im.coreInformerFactory.Core().V1().PersistentVolumes().Informer()
	}

	im.pvSynced = im.pvInformer.HasSynced

	_, err := im.pvInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    add,
			UpdateFunc: update,
			DeleteFunc: remove,
		},
	)
	if err != nil {
		return fmt.Errorf("add event handler on PersistentVolume listener: %v", err)
	}

	return nil
}

// AddVSCListener registers handlers on the VolumeSnapshotContent informer when snapshot support is configured.
//
// It is a no-op when the snapshot factory is nil. Otherwise behavior matches AddPVListener.
func (im *InformerManager) AddVSCListener(add AddFunc, update UpdateFunc, remove RemoveFunc) error {
	if im.snapshotInformerFactory == nil {
		return nil
	}
	if im.vscInformer == nil {
		im.vscInformer = im.snapshotInformerFactory.Snapshot().V1().VolumeSnapshotContents().Informer()
	}

	im.vscSynced = im.vscInformer.HasSynced

	_, err := im.vscInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    add,
			UpdateFunc: update,
			DeleteFunc: remove,
		},
	)
	if err != nil {
		return fmt.Errorf("add event handler on VolumeSnapshotContent listener: %v", err)
	}

	return nil
}

// GetPVLister returns the PersistentVolume lister associated with the core informer factory.
func (im *InformerManager) GetPVLister() corelisters.PersistentVolumeLister {
	return im.coreInformerFactory.Core().V1().PersistentVolumes().Lister()
}

// GetVSCLister returns the VolumeSnapshotContent lister, or nil when snapshot informers are disabled.
func (im *InformerManager) GetVSCLister() snapshotlisters.VolumeSnapshotContentLister {
	if im.snapshotInformerFactory == nil {
		return nil
	}
	return im.snapshotInformerFactory.Snapshot().V1().VolumeSnapshotContents().Lister()
}

// Start runs SharedInformerFactory.Run until ctx is done and blocks until caches are synced.
//
// ctx cancels informer runs. log records cache sync status.
func (im *InformerManager) Start(ctx context.Context, log zerolog.Logger) error {
	im.coreInformerFactory.Start(ctx.Done())
	if im.snapshotInformerFactory != nil {
		im.snapshotInformerFactory.Start(ctx.Done())
	}

	cacheSyncs := make([]cache.InformerSynced, 0)
	if im.pvSynced != nil {
		cacheSyncs = append(cacheSyncs, im.pvSynced)
	}
	if im.vscSynced != nil {
		cacheSyncs = append(cacheSyncs, im.vscSynced)
	}

	if len(cacheSyncs) == 0 {
		log.Warn().Msg("no Kubernetes informers registered")
		return nil
	}

	synced := cache.WaitForCacheSync(ctx.Done(), cacheSyncs...)
	if !synced {
		return fmt.Errorf("kubernetes informer cache sync incomplete")
	}
	log.Info().Msg("Kubernetes informer caches synced")
	return nil
}
