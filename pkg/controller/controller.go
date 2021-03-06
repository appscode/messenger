package controller

import (
	"fmt"

	crdutils "github.com/appscode/kutil/apiextensions/v1beta1"
	"github.com/appscode/kutil/tools/queue"
	api "github.com/appscode/messenger/apis/messenger/v1alpha1"
	cs "github.com/appscode/messenger/client/clientset/versioned"
	messengerinformers "github.com/appscode/messenger/client/informers/externalversions"
	messenger_listers "github.com/appscode/messenger/client/listers/messenger/v1alpha1"
	"github.com/golang/glog"
	crd_api "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

type MessengerController struct {
	config

	kubeClient      kubernetes.Interface
	messengerClient cs.Interface
	crdClient       crd_cs.ApiextensionsV1beta1Interface
	recorder        record.EventRecorder

	kubeInformerFactory      informers.SharedInformerFactory
	messengerInformerFactory messengerinformers.SharedInformerFactory

	// Notification
	messageQueue    *queue.Worker
	messageInformer cache.SharedIndexInformer
	messageLister   messenger_listers.MessageLister
}

func (c *MessengerController) ensureCustomResourceDefinitions() error {
	crds := []*crd_api.CustomResourceDefinition{
		api.MessagingService{}.CustomResourceDefinition(),
		api.Message{}.CustomResourceDefinition(),
	}
	return crdutils.RegisterCRDs(c.crdClient, crds)
}

func (c *MessengerController) RunInformers(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()

	glog.Info("Starting Messenger controller")
	c.kubeInformerFactory.Start(stopCh)
	c.messengerInformerFactory.Start(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	for _, v := range c.kubeInformerFactory.WaitForCacheSync(stopCh) {
		if !v {
			runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
			return
		}
	}
	for _, v := range c.messengerInformerFactory.WaitForCacheSync(stopCh) {
		if !v {
			runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
			return
		}
	}

	go c.garbageCollect(stopCh, c.GarbageCollectTime)
	c.messageQueue.Run(stopCh)
}
