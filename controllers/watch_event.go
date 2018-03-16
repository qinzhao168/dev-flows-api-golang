package controllers

import (
	"fmt"
	"time"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"github.com/golang/glog"
	"dev-flows-api-golang/modules/log"
	"dev-flows-api-golang/models"
	"dev-flows-api-golang/modules/client"

	"strings"
)

func init() {

	//client.Initk8sClient()
	stopCh := make(chan struct{})
	//go WatchEventAndPutMessageToEs(client.KubernetesClientSet.Clientset)
	go NewController(client.KubernetesClientSet.Clientset).Run(10, stopCh)

}

// Controller pod wather controller
type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

// NewController reconstruct controller
func NewController(clientset *kubernetes.Clientset) *Controller {
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "events", v1.NamespaceAll, fields.Everything())
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Event{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {

			eventInfo, ok := obj.(*v1.Event)
			if ok {
				if strings.Contains(eventInfo.GetName(), "cifid-") && strings.Contains(eventInfo.GetName(), "-cisid-") {

					var indexLog log.IndexLogData
					logClient, err := log.NewESClient("")
					if logClient == nil || err != nil {
						glog.Errorf("NewESClient failed: %v\n", err)
						return
					}
					indexLog.Log = eventInfo.Message
					indexLog.TimeNano = fmt.Sprintf("%d", time.Now().UnixNano())
					indexLog.Timestamp = eventInfo.FirstTimestamp.Add(-8 * time.Hour)
					indexLog.Kubernetes.PodName = eventInfo.InvolvedObject.Name
					indexLog.Kubernetes.Labels.ClusterID = client.ClusterID
					indexLog.Kubernetes.ContainerName = models.SCM_CONTAINER_NAME
					indexLog.Kubernetes.NamespaceName = eventInfo.InvolvedObject.Namespace
					indexLog.Kubernetes.PodId = fmt.Sprintf("%s", eventInfo.InvolvedObject.UID)
					indexEs := fmt.Sprintf("logstash-%s", time.Now().Format("2006.01.02"))
					esType := "fluentd"
					err = logClient.IndexLogToES(indexEs, esType, indexLog)
					if err != nil {
						glog.Errorf("IndexLogToES failed: %v\n", err)
					}
				}
			} else {
				glog.Infof("obj is not event:%v\n", obj)
			}
		},
	}, cache.Indexers{})

	return &Controller{indexer, queue, informer}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.syncToStdout(key.(string))
	c.handleErr(err, key)
	return true
}

// syncToStdout is the business logic of the controller. In this controller it simply prints
// information about the pod to stdout. In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *Controller) syncToStdout(key string) error {
	_, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		glog.Errorf("Fetching object with key %v from store failed with %v", key, err)
		return err
	}
	if !exists {
		glog.Infof("Event %v was deleted  %v ", key)
	}
	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
}

// Run start sync pod to local from k8s cache
func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func WatchEventAndPutMessageToEs(clientset *kubernetes.Clientset) {
	method := "WatchEventAndPutMessageToEs"
GoOn:
	var indexLog log.IndexLogData

	options := metav1.ListOptions{
		Watch: true,
	}

	// 请求watch api监听pod发生的事件
	watchInterface, err := clientset.Events(v1.NamespaceAll).Watch(options)
	if err != nil {
		glog.Errorf("WatchEventAndPutMessageToEs get event watchinterface failed===>%v\n", method, err)
		goto GoOn
	}

	for {
		select {
		case event, isOpen := <-watchInterface.ResultChan():
			if !isOpen {
				glog.Infof("%s the event watch the chan is closed\n", method)
				goto GoOn
			}
			eventInfo, ok := event.Object.(*v1.Event)
			if ok {
				if strings.Contains(eventInfo.GetName(), "cifid-") && strings.Contains(eventInfo.GetName(), "-cisid-") {

					logClient, err := log.NewESClient("")
					if logClient == nil || err != nil {
						glog.Errorf("NewESClient failed: %v\n", err)
						goto GoOn
					}
					indexLog.Log = eventInfo.Message
					indexLog.TimeNano = fmt.Sprintf("%d", time.Now().UnixNano())
					indexLog.Timestamp = eventInfo.FirstTimestamp.Add(-8 * time.Hour)
					indexLog.Kubernetes.PodName = eventInfo.InvolvedObject.Name
					indexLog.Kubernetes.Labels.ClusterID = client.ClusterID
					indexLog.Kubernetes.ContainerName = models.SCM_CONTAINER_NAME
					indexLog.Kubernetes.NamespaceName = eventInfo.InvolvedObject.Namespace
					indexLog.Kubernetes.PodId = fmt.Sprintf("%s", eventInfo.InvolvedObject.UID)
					indexEs := fmt.Sprintf("logstash-%s", time.Now().Format("2006.01.02"))
					esType := "fluentd"
					err = logClient.IndexLogToES(indexEs, esType, indexLog)
					if err != nil {
						glog.Errorf("IndexLogToES failed: %v\n", err)

						continue
					}
				}
				continue
			} else {
				glog.Infof("obj is not event:%v\n", eventInfo)
				continue
			}

		}

	}

}
