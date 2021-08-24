package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	eventv1 "k8s.io/api/events/v1"
	eventbetav1 "k8s.io/api/events/v1beta1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/events/v1"
	"k8s.io/client-go/informers/events/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	EventNameSpace = "default"

	ImagePulled = "Pulled"
)

type Controller struct {
	cancelFunc        context.CancelFunc
	StartTime         time.Time
	Desired           int32
	ImageName         string
	cache             *Cache
	ClientSet         *kubernetes.Clientset
	InformerFactory   informers.SharedInformerFactory
	BetaEvent         bool
	EventInformer     v1.EventInformer
	BetaEventInformer v1beta1.EventInformer
	EventSynced       cache.InformerSynced
	eventQueue        workqueue.RateLimitingInterface
}

type Cache struct {
	Actual int32
	sync.RWMutex
}

func (c *Cache) Get() int32 {
	c.RLock()
	defer c.RUnlock()
	return c.Actual
}

func (c *Cache) Incr() {
	c.Lock()
	defer c.Unlock()
	c.Actual += 1
}

func NewController(ctx context.Context, desired int32, image string) (*Controller, context.Context) {
	c := &Controller{
		Desired:    desired,
		ImageName:  image,
		ClientSet:  ConnectToCluster(),
		eventQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "event"),
		cache:      &Cache{},
	}
	groups, _, err := c.ClientSet.ServerGroupsAndResources()
	if err != nil {
		panic(err)
	}
	betaEvent := func() bool {
		betaEvent := false
		for _, group := range groups {
			if group.Name == "events.k8s.io" {
				if group.PreferredVersion.Version == "v1beta1" {
					betaEvent = true
				}
				break
			}
		}
		return betaEvent
	}()

	c.InformerFactory = informers.NewSharedInformerFactory(c.ClientSet, 0)
	c.BetaEvent = betaEvent
	if betaEvent {
		c.BetaEventInformer = c.InformerFactory.Events().V1beta1().Events()
		c.BetaEventInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: c.FilterEvent,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: c.AddEvent,
			},
		})
		c.EventSynced = c.BetaEventInformer.Informer().HasSynced
	} else {
		c.EventInformer = c.InformerFactory.Events().V1().Events()
		c.EventInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: c.FilterEvent,
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: c.AddEvent,
			},
		})
		c.EventSynced = c.EventInformer.Informer().HasSynced
	}
	newCtx, fn := context.WithCancel(ctx)
	c.cancelFunc = fn
	return c, newCtx
}

func (c *Controller) Run(ctx context.Context) error {
	defer c.eventQueue.ShuttingDown()
	defer func(before time.Time) {
		fmt.Printf("Spent %v. Desired=%v. Got=%v\n", time.Since(before), c.Desired, c.cache.Get())
	}(time.Now())

	if ok := cache.WaitForCacheSync(ctx.Done(), c.EventSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	if err := CreateDeployment(c.ImageName, c.Desired, c.ClientSet); err != nil {
		sugar.Errorf("Can't create deployment. Err=%v", err)
		os.Exit(1)
	}

	go wait.Until(c.runEventWorker, time.Second, ctx.Done())
	<-ctx.Done()
	return nil
}

func (c *Controller) reconcileEvent(name string) error {
	var (
		reason    string
		eventName string
	)
	if c.BetaEvent {
		event, err := c.BetaEventInformer.Lister().Events(EventNameSpace).Get(name)
		if err != nil {
			return err
		}
		reason = event.Reason
		eventName = event.Name
	} else {
		event, err := c.EventInformer.Lister().Events(EventNameSpace).Get(name)
		if err != nil {
			return err
		}
		reason = event.Reason
		eventName = event.Name
	}
	//sugar.Info(eventName, Deployment, reason)
	if reason == ImagePulled && strings.Contains(eventName, Deployment) {
		//sugar.Info(ToJsonString(event))
		c.cache.Incr()
		if c.cache.Get() == c.Desired {
			c.cancelFunc()
		}
	}
	return nil
}

func (c *Controller) runEventWorker() {
	for c.processNextEvent() {
	}
}

func (c *Controller) processNextEvent() bool {
	obj, shutdown := c.eventQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.eventQueue.Done(obj)
		var (
			key string
			ok  bool
		)
		if key, ok = obj.(string); !ok {
			c.eventQueue.Forget(obj)
			return nil
		}
		if err := c.reconcileEvent(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors
			c.eventQueue.Add(key)
			return fmt.Errorf("fail to sync '%v': %v, requeuing", key, err)
		}
		c.eventQueue.Forget(obj)
		sugar.Debug("succeed to sync '%v'", key)
		return nil
	}(obj)

	if err != nil {
		sugar.Error(err)
	}
	return true
}

func (c *Controller) FilterEvent(obj interface{}) bool {
	var (
		namespace string
	)
	if c.BetaEvent {
		betaEvent, ok := obj.(*eventbetav1.Event)
		if !ok {
			return false
		}
		namespace = betaEvent.Namespace
	} else {
		event, ok := obj.(*eventv1.Event)
		if !ok {
			return false
		}
		namespace = event.Namespace
	}
	if namespace != EventNameSpace {
		return false
	}
	return true
}

func (c *Controller) AddEvent(obj interface{}) {
	if c.BetaEvent {
		betaEvent, _ := obj.(*eventbetav1.Event)
		c.enqueueEvent(betaEvent.Name)
	} else {
		event, _ := obj.(*eventv1.Event)
		c.enqueueEvent(event.Name)
	}
}

func (c *Controller) enqueueEvent(name string) {
	c.eventQueue.Add(name)
}
